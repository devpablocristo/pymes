package helpers

import (
	"fmt"

	publicapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler/dto"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/google/uuid"
)

func FiscalCredentialCSRInput(input publicapi.FiscalCredentialCSRInput) domain.FiscalCredentialCSRInput {
	return domain.FiscalCredentialCSRInput{
		CUIT:        input.Cuit,
		LegalName:   input.LegalName,
		CommonName:  input.CommonName,
		Environment: domain.FiscalEnvironment(input.Environment),
	}
}

func FiscalCertificateUpload(input publicapi.FiscalCertificateUpload) domain.FiscalCertificateUpload {
	certificatePEM := ""
	if input.CertificatePem != nil {
		certificatePEM = *input.CertificatePem
	}
	return domain.FiscalCertificateUpload{
		CertificatePEM:  certificatePEM,
		ExpectedVersion: input.ExpectedVersion,
	}
}

func PublicFiscalCredential(input domain.FiscalCredential) (publicapi.FiscalCredential, error) {
	credentialID, err := uuid.Parse(input.ID)
	if err != nil {
		return publicapi.FiscalCredential{}, fmt.Errorf("invalid fiscal credential ID: %w", err)
	}
	return publicapi.FiscalCredential{
		CertificateExpiresAt:    input.CertificateExpiresAt,
		CertificateFingerprint:  input.CertificateFingerprint,
		CertificateSerialNumber: input.CertificateSerialNumber,
		CertificateValidFrom:    input.CertificateValidFrom,
		CommonName:              input.CommonName,
		CreatedAt:               &input.CreatedAt,
		Cuit:                    input.CUIT,
		Environment:             publicapi.FiscalCredentialEnvironment(input.Environment),
		Id:                      credentialID,
		LegalName:               input.LegalName,
		OrganizationId:          input.OrganizationID,
		Status:                  publicapi.FiscalCredentialStatus(input.Status),
		UpdatedAt:               &input.UpdatedAt,
		Version:                 input.Version,
	}, nil
}

func PublicFiscalCredentialCSRResult(
	input domain.FiscalCredentialCSRResult,
) (publicapi.FiscalCredentialCSRResult, error) {
	credential, err := PublicFiscalCredential(input.Credential)
	if err != nil {
		return publicapi.FiscalCredentialCSRResult{}, err
	}
	return publicapi.FiscalCredentialCSRResult{
		Credential: credential,
		CsrPem:     &input.CSRPEM,
	}, nil
}

func PublicFiscalPointOfSale(input domain.FiscalPointOfSale) (publicapi.FiscalPointOfSale, error) {
	credentialID, err := uuid.Parse(input.CredentialID)
	if err != nil {
		return publicapi.FiscalPointOfSale{}, fmt.Errorf("invalid fiscal credential ID: %w", err)
	}
	return publicapi.FiscalPointOfSale{
		CredentialId:   credentialID,
		Enabled:        input.Enabled,
		Environment:    publicapi.FiscalPointOfSaleEnvironment(input.Environment),
		Number:         input.Number,
		OrganizationId: input.OrganizationID,
		ValidatedAt:    input.ValidatedAt,
	}, nil
}
