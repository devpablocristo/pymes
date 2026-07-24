// Package storage selects the local development adapter or the production
// AWS-compatible KMS/S3 adapters for all fiscal process entry points.
package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/awsstore"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/securestore"
)

const (
	BackendLocal = "local"
	BackendAWS   = "aws"

	defaultLocalDirectory = "tmp/fiscal"
	// This well-known value is intentionally limited to development and test.
	defaultLocalMasterKey = "cHltZXMtdjItZGV2ZWxvcG1lbnQta2V5LTMyYnl0ZSE="
	defaultS3Prefix       = "pymes-v2"
)

type Getenv func(string) string

type Config struct {
	ApplicationEnvironment string
	Backend                string

	LocalDirectory       string
	LocalMasterKeyBase64 string

	AWSRegion        string
	KMSEndpoint      string
	KMSKeyID         string
	S3Endpoint       string
	S3Bucket         string
	S3Prefix         string
	S3ForcePathStyle bool
}

type Dependencies struct {
	KMS          fiscal.KMS
	Objects      fiscal.ObjectStore
	KeyReference string
}

func LoadConfig(getenv Getenv, applicationEnvironment string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("fiscal storage environment reader is required")
	}
	applicationEnvironment = strings.ToLower(strings.TrimSpace(applicationEnvironment))
	if applicationEnvironment == "" {
		applicationEnvironment = "development"
	}
	switch applicationEnvironment {
	case "development", "test", "production":
	default:
		return Config{}, errors.New(
			"PYMES_ENVIRONMENT must be development, test, or production",
		)
	}
	backend := strings.ToLower(strings.TrimSpace(getenv("PYMES_FISCAL_STORAGE_BACKEND")))
	if backend == "" {
		if applicationEnvironment == "production" {
			return Config{}, errors.New(
				"PYMES_FISCAL_STORAGE_BACKEND=aws is required in production",
			)
		}
		backend = BackendLocal
	}
	cfg := Config{
		ApplicationEnvironment: applicationEnvironment,
		Backend:                backend,
	}
	switch backend {
	case BackendLocal:
		if applicationEnvironment == "production" {
			return Config{}, errors.New(
				"local fiscal storage is forbidden in production; configure AWS-compatible KMS and S3",
			)
		}
		cfg.LocalDirectory = valueOrDefault(
			getenv("PYMES_FISCAL_STORAGE_DIR"),
			defaultLocalDirectory,
		)
		cfg.LocalMasterKeyBase64 = valueOrDefault(
			getenv("PYMES_FISCAL_MASTER_KEY"),
			defaultLocalMasterKey,
		)
		if _, err := securestore.DecodeMasterKey(cfg.LocalMasterKeyBase64); err != nil {
			return Config{}, fmt.Errorf("PYMES_FISCAL_MASTER_KEY: %w", err)
		}
	case BackendAWS:
		cfg.AWSRegion = firstValue(
			getenv("PYMES_FISCAL_AWS_REGION"),
			getenv("AWS_REGION"),
			getenv("AWS_DEFAULT_REGION"),
		)
		cfg.KMSEndpoint = strings.TrimSpace(getenv("PYMES_FISCAL_KMS_ENDPOINT"))
		cfg.KMSKeyID = strings.TrimSpace(getenv("PYMES_FISCAL_KMS_KEY_ID"))
		cfg.S3Endpoint = strings.TrimSpace(getenv("PYMES_FISCAL_S3_ENDPOINT"))
		cfg.S3Bucket = strings.TrimSpace(getenv("PYMES_FISCAL_S3_BUCKET"))
		cfg.S3Prefix = valueOrDefault(
			getenv("PYMES_FISCAL_S3_PREFIX"),
			defaultS3Prefix,
		)
		var err error
		cfg.S3ForcePathStyle, err = boolOrDefault(
			getenv("PYMES_FISCAL_S3_FORCE_PATH_STYLE"),
			false,
		)
		if err != nil {
			return Config{}, fmt.Errorf("PYMES_FISCAL_S3_FORCE_PATH_STYLE: %w", err)
		}
		if cfg.AWSRegion == "" || cfg.KMSKeyID == "" || cfg.S3Bucket == "" {
			return Config{}, errors.New(
				"PYMES_FISCAL_AWS_REGION, PYMES_FISCAL_KMS_KEY_ID and PYMES_FISCAL_S3_BUCKET are required for AWS fiscal storage",
			)
		}
		for name, endpoint := range map[string]string{
			"PYMES_FISCAL_KMS_ENDPOINT": cfg.KMSEndpoint,
			"PYMES_FISCAL_S3_ENDPOINT":  cfg.S3Endpoint,
		} {
			if err := validateEndpoint(endpoint, applicationEnvironment == "production"); err != nil {
				return Config{}, fmt.Errorf("%s: %w", name, err)
			}
		}
	default:
		return Config{}, errors.New(
			"PYMES_FISCAL_STORAGE_BACKEND must be local or aws",
		)
	}
	return cfg, nil
}

// Open validates both managed services before returning. Production processes
// therefore never start with a missing/inaccessible KMS key or S3 bucket.
func Open(ctx context.Context, cfg Config) (Dependencies, error) {
	switch cfg.Backend {
	case BackendLocal:
		if cfg.ApplicationEnvironment == "production" {
			return Dependencies{}, errors.New("local fiscal storage is forbidden in production")
		}
		masterKey, err := securestore.DecodeMasterKey(cfg.LocalMasterKeyBase64)
		if err != nil {
			return Dependencies{}, err
		}
		store, err := securestore.New(cfg.LocalDirectory, masterKey)
		if err != nil {
			return Dependencies{}, err
		}
		return Dependencies{
			KMS:          store,
			Objects:      store,
			KeyReference: "secret://local/root",
		}, nil
	case BackendAWS:
		if cfg.AWSRegion == "" || cfg.KMSKeyID == "" || cfg.S3Bucket == "" {
			return Dependencies{}, errors.New("incomplete AWS fiscal storage configuration")
		}
		awsConfig, err := awsconfig.LoadDefaultConfig(
			ctx,
			awsconfig.WithRegion(cfg.AWSRegion),
		)
		if err != nil {
			return Dependencies{}, fmt.Errorf("load AWS fiscal storage configuration: %w", err)
		}
		s3Client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
			options.UsePathStyle = cfg.S3ForcePathStyle
			if cfg.S3Endpoint != "" {
				options.BaseEndpoint = aws.String(cfg.S3Endpoint)
			}
		})
		objects, err := awsstore.NewS3Store(
			s3Client,
			cfg.S3Bucket,
			cfg.S3Prefix,
		)
		if err != nil {
			return Dependencies{}, err
		}
		kmsClient := kms.NewFromConfig(awsConfig, func(options *kms.Options) {
			if cfg.KMSEndpoint != "" {
				options.BaseEndpoint = aws.String(cfg.KMSEndpoint)
			}
		})
		kmsStore, err := awsstore.NewKMSStore(kmsClient, cfg.KMSKeyID, objects)
		if err != nil {
			return Dependencies{}, err
		}
		validationContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := objects.Validate(validationContext); err != nil {
			return Dependencies{}, err
		}
		if err := kmsStore.Validate(validationContext); err != nil {
			return Dependencies{}, err
		}
		return Dependencies{
			KMS:          kmsStore,
			Objects:      objects,
			KeyReference: kmsStore.RootReference(),
		}, nil
	default:
		return Dependencies{}, errors.New("unsupported fiscal storage backend")
	}
}

func validateEndpoint(raw string, requireTLS bool) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("must be an absolute HTTP(S) URL")
	}
	if requireTLS && parsed.Scheme != "https" {
		return errors.New("must use HTTPS in production")
	}
	if parsed.User != nil {
		return errors.New("must not embed credentials")
	}
	return nil
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func boolOrDefault(value string, fallback bool) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New("must be true or false")
	}
	return parsed, nil
}
