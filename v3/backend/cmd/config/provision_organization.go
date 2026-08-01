package config

import (
	"errors"
	"os"
	"strings"
)

type ProvisionOrganizationConfig struct {
	DatabaseURL                string
	AccountingProvisioningURL  string
	Environment                string
	AllowInsecureLocalServices bool
}

type ProvisionOrganizationConfigError struct {
	Code string
	Err  error
}

func (err *ProvisionOrganizationConfigError) Error() string {
	return err.Err.Error()
}

func (err *ProvisionOrganizationConfigError) Unwrap() error {
	return err.Err
}

func LoadProvisionOrganization() (ProvisionOrganizationConfig, error) {
	return LoadProvisionOrganizationFrom(os.Getenv)
}

func LoadProvisionOrganizationFrom(
	getenv func(string) string,
) (ProvisionOrganizationConfig, error) {
	if getenv == nil {
		return ProvisionOrganizationConfig{}, provisionOrganizationConfigError(
			"PROVISION_CONFIG_INVALID",
			"environment reader is required",
		)
	}
	environment := strings.ToLower(
		defaultValue(getenv("PYMES_ENVIRONMENT"), "development"),
	)
	if environment != "development" &&
		environment != "test" &&
		environment != "production" {
		return ProvisionOrganizationConfig{}, provisionOrganizationConfigError(
			"WORKLOAD_IDENTITY_INVALID",
			"PYMES_ENVIRONMENT must be development, test, or production",
		)
	}
	allowInsecure := strings.EqualFold(
		strings.TrimSpace(getenv("PYMES_ALLOW_INSECURE_LOCAL_SERVICES")),
		"true",
	)
	if allowInsecure && environment == "production" {
		return ProvisionOrganizationConfig{}, provisionOrganizationConfigError(
			"WORKLOAD_IDENTITY_INVALID",
			"insecure local platform identity is forbidden in production",
		)
	}
	databaseURL := strings.TrimSpace(getenv("PYMES_DATABASE_URL"))
	if databaseURL == "" {
		return ProvisionOrganizationConfig{}, provisionOrganizationConfigError(
			"DATABASE_URL_MISSING",
			"PYMES_DATABASE_URL is required",
		)
	}
	accountingURL := strings.TrimSpace(
		getenv("ACCOUNTING_PROVISIONING_URL"),
	)
	if accountingURL == "" {
		return ProvisionOrganizationConfig{}, provisionOrganizationConfigError(
			"DEPENDENCY_URL_MISSING",
			"ACCOUNTING_PROVISIONING_URL is required",
		)
	}
	return ProvisionOrganizationConfig{
		DatabaseURL:                databaseURL,
		AccountingProvisioningURL:  accountingURL,
		Environment:                environment,
		AllowInsecureLocalServices: allowInsecure,
	}, nil
}

func ProvisionOrganizationErrorCode(err error) string {
	var configErr *ProvisionOrganizationConfigError
	if errors.As(err, &configErr) && configErr.Code != "" {
		return configErr.Code
	}
	return "PROVISION_CONFIG_INVALID"
}

func provisionOrganizationConfigError(
	code string,
	message string,
) error {
	return &ProvisionOrganizationConfigError{
		Code: code,
		Err:  errors.New(message),
	}
}
