package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	"github.com/google/uuid"
)

type CredentialProvider struct {
	db      DBTX
	objects fiscal.ObjectStore
}

func NewCredentialProvider(db DBTX, objects fiscal.ObjectStore) (*CredentialProvider, error) {
	if db == nil || objects == nil {
		return nil, errors.New("fiscal credential database and object store are required")
	}
	return &CredentialProvider{db: db, objects: objects}, nil
}

func (provider *CredentialProvider) Credentials(
	ctx context.Context,
	voucher fiscal.Voucher,
) (wsaa.Credentials, error) {
	if voucher.OrganizationID == uuid.Nil ||
		(voucher.Environment != string(ar.Homologation) &&
			voucher.Environment != string(ar.Production)) {
		return wsaa.Credentials{}, errors.New("valid fiscal voucher scope is required")
	}
	var (
		cuitRaw        string
		certificateRef string
		keyReference   string
	)
	repository := &Repository{db: provider.db}
	err := repository.withinTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		if err := bindOrganization(txContext, tx, voucher.OrganizationID); err != nil {
			return err
		}
		return tx.QueryRow(txContext, `
			SELECT
				settings.cuit::text,
				certificate.certificate_ref,
				certificate.private_key_ref
			  FROM fiscal_ar.settings AS settings
			  JOIN fiscal.certificates AS certificate
			    ON certificate.org_id = settings.org_id
			   AND certificate.environment = settings.environment
			 WHERE settings.org_id = $1
			   AND settings.environment = $2
			   AND settings.enabled
			   AND certificate.status = 'active'
			   AND certificate.valid_from <= now()
			   AND certificate.valid_until > now() + interval '5 minutes'
			 ORDER BY certificate.valid_until DESC, certificate.id DESC
			 LIMIT 1`,
			voucher.OrganizationID,
			voucher.Environment,
		).Scan(&cuitRaw, &certificateRef, &keyReference)
	})
	if err != nil {
		return wsaa.Credentials{}, fmt.Errorf("load active fiscal credentials: %w", mapDatabaseError(err))
	}
	cuit, err := ar.ParseCUIT(cuitRaw)
	if err != nil {
		return wsaa.Credentials{}, fmt.Errorf("parse fiscal credential CUIT: %w", err)
	}
	certificateObject, err := provider.objects.Get(ctx, strings.TrimSpace(certificateRef))
	if err != nil {
		return wsaa.Credentials{}, fmt.Errorf("load public fiscal certificate: %w", err)
	}
	if len(certificateObject.Body) == 0 {
		return wsaa.Credentials{}, errors.New("stored public fiscal certificate is empty")
	}
	return wsaa.Credentials{
		OrganizationID: voucher.OrganizationID,
		Environment:    ar.Environment(voucher.Environment),
		Service:        wsaa.ServiceWSFE,
		CUIT:           cuit,
		CertificatePEM: append([]byte(nil), certificateObject.Body...),
		KeyReference:   strings.TrimSpace(keyReference),
	}, nil
}
