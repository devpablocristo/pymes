package helpers

import (
	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func CredentialCSRRequest(input domain.FiscalCredentialCSRInput) fiscalapi.CSRRequest {
	return fiscalapi.CSRRequest{
		CommonName:  input.CommonName,
		Cuit:        input.CUIT,
		Environment: fiscalapi.CSRRequestEnvironment(input.Environment),
		LegalName:   input.LegalName,
	}
}

func CertificateUpload(input domain.FiscalCertificateUpload) fiscalapi.CertificateUpload {
	return fiscalapi.CertificateUpload{
		CertificatePem:  input.CertificatePEM,
		ExpectedVersion: input.ExpectedVersion,
	}
}

func CredentialCSRResult(input fiscalapi.CSRResult) domain.FiscalCredentialCSRResult {
	return domain.FiscalCredentialCSRResult{
		Credential: Credential(input.Credential),
		CSRPEM:     input.CsrPem,
	}
}

func Credential(input fiscalapi.FiscalCredential) domain.FiscalCredential {
	return domain.FiscalCredential{
		ID:                      input.Id,
		OrganizationID:          input.OrganizationId,
		CUIT:                    input.Cuit,
		LegalName:               input.LegalName,
		CommonName:              input.CommonName,
		Environment:             domain.FiscalEnvironment(input.Environment),
		Status:                  domain.FiscalCredentialStatus(input.Status),
		Version:                 input.Version,
		CertificateFingerprint:  input.CertificateFingerprint,
		CertificateSerialNumber: input.CertificateSerialNumber,
		CertificateValidFrom:    input.CertificateValidFrom,
		CertificateExpiresAt:    input.CertificateExpiresAt,
		CreatedAt:               input.CreatedAt,
		UpdatedAt:               input.UpdatedAt,
	}
}

func PointOfSale(input fiscalapi.FiscalPointOfSale) domain.FiscalPointOfSale {
	return domain.FiscalPointOfSale{
		OrganizationID: input.OrganizationId,
		CredentialID:   input.CredentialId,
		Environment:    domain.FiscalEnvironment(input.Environment),
		Number:         input.Number,
		Enabled:        input.Enabled,
		ValidatedAt:    input.ValidatedAt,
	}
}
