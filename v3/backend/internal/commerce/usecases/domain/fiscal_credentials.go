package domain

import "time"

type FiscalEnvironment string

const (
	FiscalEnvironmentHomologation FiscalEnvironment = "homologation"
	FiscalEnvironmentProduction   FiscalEnvironment = "production"
)

func (e FiscalEnvironment) Valid() bool {
	return e == FiscalEnvironmentHomologation || e == FiscalEnvironmentProduction
}

type FiscalCredentialStatus string

const (
	FiscalCredentialPendingCertificate FiscalCredentialStatus = "pending_certificate"
	FiscalCredentialReady              FiscalCredentialStatus = "ready"
	FiscalCredentialExpired            FiscalCredentialStatus = "expired"
	FiscalCredentialDisabled           FiscalCredentialStatus = "disabled"
)

type FiscalCredentialCSRInput struct {
	CUIT        string
	LegalName   string
	CommonName  string
	Environment FiscalEnvironment
}

func (v FiscalCredentialCSRInput) Valid() bool {
	return validCUIT(v.CUIT) &&
		v.LegalName != "" &&
		v.CommonName != "" &&
		v.Environment.Valid()
}

type FiscalCredential struct {
	ID                      string
	OrganizationID          string
	CUIT                    string
	LegalName               string
	CommonName              string
	Environment             FiscalEnvironment
	Status                  FiscalCredentialStatus
	Version                 int
	CertificateFingerprint  *string
	CertificateSerialNumber *string
	CertificateValidFrom    *time.Time
	CertificateExpiresAt    *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type FiscalCredentialCSRResult struct {
	Credential FiscalCredential
	CSRPEM     string
}

type FiscalCertificateUpload struct {
	CertificatePEM  string
	ExpectedVersion int
}

func (v FiscalCertificateUpload) Valid() bool {
	return v.CertificatePEM != "" && v.ExpectedVersion > 0
}

type FiscalPointOfSale struct {
	OrganizationID string
	CredentialID   string
	Environment    FiscalEnvironment
	Number         int
	Enabled        bool
	ValidatedAt    *time.Time
}

func validCUIT(value string) bool {
	if len(value) != 11 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
