// Package awsstore provides production fiscal storage adapters backed by
// AWS-compatible KMS and S3 APIs.
package awsstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

const maxObjectSize = 32 << 20

type S3API interface {
	PutObject(
		context.Context,
		*s3.PutObjectInput,
		...func(*s3.Options),
	) (*s3.PutObjectOutput, error)
	GetObject(
		context.Context,
		*s3.GetObjectInput,
		...func(*s3.Options),
	) (*s3.GetObjectOutput, error)
	HeadBucket(
		context.Context,
		*s3.HeadBucketInput,
		...func(*s3.Options),
	) (*s3.HeadBucketOutput, error)
}

// S3Store uses conditional creates, so a logical fiscal object can never be
// overwritten. SHA-256 is checked both before upload and after download.
type S3Store struct {
	client S3API
	bucket string
	prefix string
}

func NewS3Store(client S3API, bucket, prefix string) (*S3Store, error) {
	bucket = strings.TrimSpace(bucket)
	if client == nil || bucket == "" {
		return nil, errors.New("S3 client and fiscal bucket are required")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		clean := path.Clean(prefix)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, errors.New("invalid fiscal S3 prefix")
		}
		prefix = clean
	}
	return &S3Store{client: client, bucket: bucket, prefix: prefix}, nil
}

func (store *S3Store) Validate(ctx context.Context) error {
	if store == nil {
		return errors.New("nil fiscal S3 store")
	}
	if _, err := store.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(store.bucket),
	}); err != nil {
		return fmt.Errorf("validate fiscal S3 bucket access: %w", err)
	}
	return nil
}

func (store *S3Store) PutImmutable(
	ctx context.Context,
	object fiscal.ImmutableObject,
) error {
	if store == nil {
		return errors.New("nil fiscal S3 store")
	}
	key, err := store.objectKey(object.Key)
	if err != nil {
		return err
	}
	contentType := strings.TrimSpace(object.ContentType)
	if contentType == "" {
		return errors.New("immutable fiscal object content type is required")
	}
	if len(object.Body) == 0 || len(object.Body) > maxObjectSize {
		return errors.New("immutable fiscal object must contain between 1 byte and 32 MiB")
	}
	sum := sha256.Sum256(object.Body)
	digest := hex.EncodeToString(sum[:])
	if object.SHA256 != "" && !strings.EqualFold(strings.TrimSpace(object.SHA256), digest) {
		return errors.New("immutable fiscal object hash does not match its body")
	}
	checksum := base64.StdEncoding.EncodeToString(sum[:])
	contentLength := int64(len(object.Body))
	_, err = store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(store.bucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(object.Body),
		ContentLength:        &contentLength,
		ContentType:          aws.String(contentType),
		ChecksumSHA256:       aws.String(checksum),
		IfNoneMatch:          aws.String("*"),
		Metadata:             map[string]string{"sha256": digest},
		ServerSideEncryption: s3types.ServerSideEncryptionAes256,
	})
	if err == nil {
		return nil
	}
	if !isS3PreconditionFailure(err) {
		return fmt.Errorf("store immutable fiscal object: %w", err)
	}
	existing, getErr := store.Get(ctx, object.Key)
	if getErr != nil {
		return fmt.Errorf("verify concurrent immutable fiscal object: %w", getErr)
	}
	if !bytes.Equal(existing.Body, object.Body) ||
		existing.ContentType != contentType ||
		!strings.EqualFold(existing.SHA256, digest) {
		return errors.New("immutable fiscal object key already contains different bytes")
	}
	return nil
}

func (store *S3Store) Get(
	ctx context.Context,
	logicalKey string,
) (fiscal.ImmutableObject, error) {
	if store == nil {
		return fiscal.ImmutableObject{}, errors.New("nil fiscal S3 store")
	}
	key, err := store.objectKey(logicalKey)
	if err != nil {
		return fiscal.ImmutableObject{}, err
	}
	output, err := store.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return fiscal.ImmutableObject{}, fiscal.ErrNotFound
		}
		return fiscal.ImmutableObject{}, fmt.Errorf("load immutable fiscal object: %w", err)
	}
	if output == nil || output.Body == nil {
		return fiscal.ImmutableObject{}, errors.New("S3 returned an empty fiscal object response")
	}
	defer output.Body.Close()
	body, err := io.ReadAll(io.LimitReader(output.Body, maxObjectSize+1))
	if err != nil {
		return fiscal.ImmutableObject{}, fmt.Errorf("read immutable fiscal object: %w", err)
	}
	if len(body) == 0 || len(body) > maxObjectSize {
		return fiscal.ImmutableObject{}, errors.New("immutable fiscal object has an invalid size")
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	storedDigest := strings.TrimSpace(output.Metadata["sha256"])
	if storedDigest == "" || !strings.EqualFold(storedDigest, digest) {
		return fiscal.ImmutableObject{}, errors.New("immutable fiscal object integrity check failed")
	}
	contentType := strings.TrimSpace(aws.ToString(output.ContentType))
	if contentType == "" {
		return fiscal.ImmutableObject{}, errors.New("immutable fiscal object content type is missing")
	}
	return fiscal.ImmutableObject{
		Key:         strings.TrimSpace(logicalKey),
		ContentType: contentType,
		Body:        body,
		SHA256:      digest,
	}, nil
}

func (store *S3Store) objectKey(logicalKey string) (string, error) {
	logicalKey = strings.TrimSpace(strings.ReplaceAll(logicalKey, "\\", "/"))
	if logicalKey == "" || strings.HasPrefix(logicalKey, "/") {
		return "", errors.New("invalid immutable fiscal object key")
	}
	clean := path.Clean(logicalKey)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("invalid immutable fiscal object key")
	}
	if store.prefix == "" {
		return clean, nil
	}
	return path.Join(store.prefix, clean), nil
}

func isS3PreconditionFailure(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) &&
		(apiErr.ErrorCode() == "PreconditionFailed" ||
			apiErr.ErrorCode() == "ConditionalRequestConflict") {
		return true
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) &&
		(responseErr.HTTPStatusCode() == 409 || responseErr.HTTPStatusCode() == 412)
}

func isS3NotFound(err error) bool {
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) &&
		(apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
		return true
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == 404
}
