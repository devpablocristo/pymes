package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	fiscalpg "github.com/devpablocristo/pymes/v2/backend/internal/fiscal/postgres"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *IAMAPI) rotateFiscalCertificate(
	w http.ResponseWriter,
	r *http.Request,
	_ api.RotateFiscalCertificateParams,
) {
	var input api.FiscalCertificateInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if !input.Environment.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal environment")
		return
	}
	importer, ok := h.fiscalKMS.(fiscal.SigningKeyImporter)
	if !ok || h.fiscalObjects == nil || strings.TrimSpace(h.fiscalKMSKeyReference) == "" {
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"FISCAL_KMS_UNAVAILABLE",
			"A signing-key KMS and immutable object store must be configured",
		)
		return
	}

	certificatePEM := []byte(input.CertificatePem)
	privateKeyPEM := []byte(input.PrivateKeyPem)
	certificate, err := ar.ParseCertificate(certificatePEM)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "FISCAL_CERTIFICATE_INVALID", err.Error())
		return
	}
	privatePublicKey, err := privateKeyPublicKey(privateKeyPEM)
	if err != nil || !publicKeysEqual(certificate.PublicKey, privatePublicKey) {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"FISCAL_CERTIFICATE_INVALID",
			"The certificate and private key do not match",
		)
		return
	}

	var response api.FiscalCertificate
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var profileCUIT string
			if err := tx.QueryRow(ctx, `
				SELECT cuit::text
				  FROM fiscal_ar.settings
				 WHERE environment = $1
			`, string(input.Environment)).Scan(&profileCUIT); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: fiscal settings", errBusinessNotFound)
				}
				return fmt.Errorf("load fiscal profile for certificate: %w", err)
			}
			expectedCUIT, err := ar.ParseCUIT(profileCUIT)
			if err != nil {
				return fmt.Errorf("parse configured CUIT: %w", err)
			}

			additionalData := []byte(active.OrganizationID + ":" + string(input.Environment))
			keyReference, err := importer.ImportSigningKey(
				ctx,
				h.fiscalKMSKeyReference,
				privateKeyPEM,
				additionalData,
			)
			if err != nil {
				return fmt.Errorf("import fiscal signing key: %w", err)
			}
			if !strings.HasPrefix(keyReference, "kms://") &&
				!strings.HasPrefix(keyReference, "vault://") &&
				!strings.HasPrefix(keyReference, "secret://") {
				return errors.New("KMS returned an invalid signing-key reference")
			}
			kmsPublicKey, err := h.fiscalKMS.PublicKey(ctx, keyReference)
			if err != nil {
				return fmt.Errorf("load imported fiscal signing key: %w", err)
			}
			info, err := ar.ValidateCertificate(
				certificatePEM,
				kmsPublicKey,
				expectedCUIT,
				h.now().UTC(),
			)
			if err != nil {
				return fmt.Errorf("validate fiscal certificate: %w", err)
			}

			orgID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active organization: %w", err)
			}
			certificateObjectKey := fmt.Sprintf(
				"fiscal/%s/certificates/%s.pem",
				orgID,
				info.Fingerprint,
			)
			certificateObjectDigest := sha256.Sum256(certificatePEM)
			if err := h.fiscalObjects.PutImmutable(ctx, fiscal.ImmutableObject{
				Key:         certificateObjectKey,
				ContentType: "application/x-pem-file",
				Body:        append([]byte(nil), certificatePEM...),
				SHA256:      hex.EncodeToString(certificateObjectDigest[:]),
			}); err != nil {
				return fmt.Errorf("store public fiscal certificate: %w", err)
			}

			var id uuid.UUID
			err = tx.QueryRow(ctx, `
				WITH rotated AS (
					UPDATE fiscal.certificates
					   SET status = 'rotated',
					       updated_at = now()
					 WHERE environment = $1
					   AND status = 'active'
					RETURNING id
				)
				INSERT INTO fiscal.certificates (
					org_id,
					country_code,
					environment,
					certificate_ref,
					private_key_ref,
					fingerprint_sha256,
					subject_tax_id,
					valid_from,
					valid_until,
					rotates_certificate_id,
					created_by
				)
				VALUES (
					$2::uuid,
					'AR',
					$1,
					$3,
					$4,
					$5,
					$6,
					$7,
					$8,
					(SELECT id FROM rotated ORDER BY id LIMIT 1),
					$9
				)
				RETURNING id
			`,
				string(input.Environment),
				active.OrganizationID,
				certificateObjectKey,
				keyReference,
				info.Fingerprint,
				info.CUIT.String(),
				info.NotBefore,
				info.NotAfter,
				active.UserID,
			).Scan(&id)
			if err != nil {
				return fmt.Errorf("store fiscal certificate reference: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				UPDATE fiscal_ar.settings
				   SET enabled = ($1 = 'homologation'),
				       version = version + 1,
				       updated_at = now()
				 WHERE environment = $1
				   AND cuit = $2`,
				string(input.Environment), info.CUIT.String(),
			); err != nil {
				return fmt.Errorf("enable fiscal environment after certificate rotation: %w", err)
			}
			response = api.FiscalCertificate{
				Active:      true,
				Environment: input.Environment,
				ExpiresAt:   info.NotAfter,
				Fingerprint: info.Fingerprint,
				Id:          id,
				ValidFrom:   info.NotBefore,
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) createFiscalCreditNote(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateFiscalCreditNoteParams,
) {
	h.createFiscalAdjustment(w, r, fiscal.OperationCreditNote, string(params.IdempotencyKey))
}

func (h *IAMAPI) createFiscalDebitNote(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateFiscalDebitNoteParams,
) {
	h.createFiscalAdjustment(w, r, fiscal.OperationDebitNote, string(params.IdempotencyKey))
}

func (h *IAMAPI) getIVASimple(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.GetIVASimpleParams,
) {
	normalizedPeriod, firstDay, nextMonth, err := fiscalPeriod(period)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	environment := string(api.GetIVASimpleParamsEnvironmentProduction)
	if params.Environment != nil {
		environment = string(*params.Environment)
	}
	response := api.IVASimpleReport{
		Period:   normalizedPeriod,
		SalesNet: "0", OutputVat: "0", PurchasesNet: "0", InputVat: "0",
		Withholdings: "0", Perceptions: "0", Balance: "0",
		ValidationErrors: make([]string, 0),
	}
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			records, totals, err := loadIVARecords(
				ctx,
				tx,
				firstDay,
				nextMonth,
				environment,
			)
			if err != nil {
				return err
			}
			response.SalesNet = totals.salesNet.String()
			response.OutputVat = totals.outputVAT.String()
			response.PurchasesNet = totals.purchasesNet.String()
			response.InputVat = totals.inputVAT.String()
			response.Withholdings = totals.withholdings.String()
			response.Perceptions = totals.perceptions.String()
			response.Balance = totals.outputVAT.
				Sub(totals.inputVAT).
				Sub(totals.withholdings).
				Sub(totals.perceptions).
				String()

			files, exportErr := ar.ExportIVASimple(strings.ReplaceAll(normalizedPeriod, "-", ""), records)
			if exportErr != nil {
				response.ValidationErrors = append(response.ValidationErrors, exportErr.Error())
				return nil
			}
			salesBundle, err := fiscalRegistryBundle(
				"ventas-comprobantes.txt", files.SalesVouchers,
				"ventas-alicuotas.txt", files.SalesVAT,
			)
			if err != nil {
				return err
			}
			purchasesBundle, err := fiscalRegistryBundle(
				"compras-comprobantes.txt", files.PurchaseVouchers,
				"compras-alicuotas.txt", files.PurchaseVAT,
			)
			if err != nil {
				return err
			}
			salesEncoded := base64.StdEncoding.EncodeToString(salesBundle)
			purchasesEncoded := base64.StdEncoding.EncodeToString(purchasesBundle)
			response.SalesFile = &salesEncoded
			response.PurchasesFile = &purchasesEncoded
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listFiscalPointsOfSale(w http.ResponseWriter, r *http.Request) {
	points := make([]api.FiscalPointOfSale, 0)
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			rows, err := tx.Query(ctx, `
				SELECT id, environment, code, name, enabled
				  FROM fiscal.points_of_sale
				 WHERE country_code = 'AR'
				 ORDER BY environment, code, id
			`)
			if err != nil {
				return fmt.Errorf("list fiscal points of sale: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var point api.FiscalPointOfSale
				var environment string
				var name string
				if err := rows.Scan(
					&point.Id,
					&environment,
					&point.Number,
					&name,
					&point.Active,
				); err != nil {
					return fmt.Errorf("scan fiscal point of sale: %w", err)
				}
				point.Environment = api.FiscalEnvironment(environment)
				point.Name = &name
				points = append(points, point)
			}
			return rows.Err()
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *IAMAPI) createFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateFiscalPointOfSaleParams,
) {
	var input api.FiscalPointOfSaleInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if !input.Environment.Valid() || input.Number < 1 || input.Number > 99999 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal point of sale")
		return
	}
	name := fmt.Sprintf("Punto de venta %05d", input.Number)
	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		name = strings.TrimSpace(*input.Name)
	}
	var response api.FiscalPointOfSale
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			err := tx.QueryRow(ctx, `
				INSERT INTO fiscal.points_of_sale (
					org_id,
					country_code,
					environment,
					code,
					issuing_system,
					name
				)
				VALUES ($1::uuid, 'AR', $2, $3, 'wsfev1', $4)
				RETURNING id, environment, code, name, enabled
			`, active.OrganizationID, string(input.Environment), input.Number, name).Scan(
				&response.Id,
				&response.Environment,
				&response.Number,
				&name,
				&response.Active,
			)
			if err != nil {
				return fmt.Errorf("create fiscal point of sale: %w", err)
			}
			response.Name = &name
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) getFiscalSettings(
	w http.ResponseWriter,
	r *http.Request,
	params api.GetFiscalSettingsParams,
) {
	environment := api.FiscalEnvironmentHomologation
	if params.Environment != nil {
		environment = *params.Environment
	}
	if !environment.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal environment")
		return
	}
	var response api.ArgentinaFiscalSettings
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var err error
			response, err = loadFiscalSettingsResponse(
				ctx,
				tx,
				environment,
				h.now().UTC(),
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) updateFiscalSettings(
	w http.ResponseWriter,
	r *http.Request,
	_ api.UpdateFiscalSettingsParams,
) {
	var input api.ArgentinaFiscalSettingsInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if !input.Environment.Valid() || !input.TaxCondition.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid Argentina fiscal settings")
		return
	}
	cuit, err := ar.ParseCUIT(input.Cuit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "FISCAL_CUIT_INVALID", err.Error())
		return
	}
	if strings.TrimSpace(input.LegalName) == "" ||
		strings.TrimSpace(input.TaxAddress) == "" ||
		input.ActivityStartDate.Time.IsZero() ||
		len(strings.TrimSpace(input.FunctionalCurrency)) != 3 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Legal name, address, activity date, and currency are required")
		return
	}
	var response api.ArgentinaFiscalSettings
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			if _, err := tx.Exec(ctx, `
				SELECT pg_advisory_xact_lock(
					hashtextextended($1, 941016)
				)`,
				active.OrganizationID,
			); err != nil {
				return fmt.Errorf("serialize fiscal settings update: %w", err)
			}
			var (
				existingVersion   int64
				existingEnabled   bool
				existingCUIT      string
				existingCondition string
			)
			err := tx.QueryRow(ctx, `
				SELECT version, enabled, cuit::text, iva_condition
				  FROM fiscal_ar.settings
				 WHERE environment = $1
				 FOR UPDATE
			`, string(input.Environment)).Scan(
				&existingVersion,
				&existingEnabled,
				&existingCUIT,
				&existingCondition,
			)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				if input.Version != 0 {
					return errBusinessVersionConflict
				}
			case err != nil:
				return fmt.Errorf("lock fiscal settings: %w", err)
			case existingVersion != input.Version:
				return errBusinessVersionConflict
			}

			address, err := json.Marshal(map[string]string{
				"formatted": strings.TrimSpace(input.TaxAddress),
			})
			if err != nil {
				return fmt.Errorf("encode fiscal address: %w", err)
			}
			activityDate := input.ActivityStartDate.Time
			var profileVersion int64
			profileErr := tx.QueryRow(ctx, `
				SELECT version
				  FROM fiscal.profiles
				 FOR UPDATE`,
			).Scan(&profileVersion)
			profileExists := profileErr == nil
			if profileErr != nil && !errors.Is(profileErr, pgx.ErrNoRows) {
				return fmt.Errorf("lock fiscal profile: %w", profileErr)
			}
			if !profileExists {
				if _, err := tx.Exec(ctx, `
					INSERT INTO fiscal.profiles (
						org_id,
						country_code,
						legal_name,
						legal_address,
						tax_condition,
						activity_start_date,
						default_currency
					)
					VALUES ($1::uuid, 'AR', $2, $3::jsonb, $4, $5, upper($6))`,
					active.OrganizationID,
					strings.TrimSpace(input.LegalName),
					string(address),
					dbTaxCondition(input.TaxCondition),
					activityDate,
					strings.ToUpper(input.FunctionalCurrency),
				); err != nil {
					return fmt.Errorf("create fiscal profile: %w", err)
				}
			}

			if existingVersion == 0 {
				err = tx.QueryRow(ctx, `
					INSERT INTO fiscal_ar.settings (
						org_id,
						environment,
						cuit,
						iva_condition,
						enabled
					)
					VALUES ($1::uuid, $2, $3, $4, false)
					RETURNING version
				`,
					active.OrganizationID,
					string(input.Environment),
					cuit.String(),
					dbTaxCondition(input.TaxCondition),
				).Scan(&response.Version)
			} else {
				keepHomologationEnabled :=
					input.Environment == api.FiscalEnvironmentHomologation &&
						existingEnabled &&
						existingCUIT == cuit.String() &&
						existingCondition == dbTaxCondition(input.TaxCondition)
				err = tx.QueryRow(ctx, `
					UPDATE fiscal_ar.settings
					   SET cuit = $2,
					       iva_condition = $3,
					       enabled = $4,
					       version = version + 1,
					       updated_at = now()
					 WHERE environment = $1
					   AND version = $5
					RETURNING version
				`,
					string(input.Environment),
					cuit.String(),
					dbTaxCondition(input.TaxCondition),
					keepHomologationEnabled,
					input.Version,
				).Scan(&response.Version)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return errBusinessVersionConflict
			}
			if err != nil {
				return fmt.Errorf("upsert Argentina fiscal settings: %w", err)
			}

			profileChanged := !profileExists
			if profileExists {
				tag, err := tx.Exec(ctx, `
					UPDATE fiscal.profiles
					   SET legal_name = $2,
					       legal_address = $3::jsonb,
					       tax_condition = $4,
					       activity_start_date = $5,
					       default_currency = upper($6),
					       version = version + 1,
					       updated_at = now()
					 WHERE org_id = $1::uuid
					   AND (
							legal_name,
							legal_address,
							tax_condition,
							activity_start_date,
							default_currency
					   ) IS DISTINCT FROM (
							$2,
							$3::jsonb,
							$4,
							$5,
							upper($6)
					   )`,
					active.OrganizationID,
					strings.TrimSpace(input.LegalName),
					string(address),
					dbTaxCondition(input.TaxCondition),
					activityDate,
					strings.ToUpper(input.FunctionalCurrency),
				)
				if err != nil {
					return fmt.Errorf("update fiscal profile: %w", err)
				}
				profileChanged = tag.RowsAffected() == 1
			}
			if profileChanged {
				if _, err := tx.Exec(ctx, `
					UPDATE fiscal_ar.settings
					   SET enabled = CASE
								WHEN environment = 'production' THEN false
								ELSE enabled
						   END,
					       version = version + 1,
					       updated_at = now()
					 WHERE environment <> $1`,
					string(input.Environment),
				); err != nil {
					return fmt.Errorf(
						"invalidate settings after fiscal profile change: %w",
						err,
					)
				}
			}

			response.CountryCode = "AR"
			response.Cuit = cuit.String()
			response.Environment = input.Environment
			response.FunctionalCurrency = strings.ToUpper(input.FunctionalCurrency)
			response.LegalName = strings.TrimSpace(input.LegalName)
			response.TaxAddress = strings.TrimSpace(input.TaxAddress)
			response.TaxCondition = input.TaxCondition
			response.ActivityStartDate = input.ActivityStartDate
			response.ProductionReady = false
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listFiscalVouchers(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListFiscalVouchersParams,
) {
	if !params.Environment.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal environment")
		return
	}
	if params.Status != nil && !params.Status.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal voucher state")
		return
	}
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	limit := normalizedLimit((*int)(params.Limit))
	query := ""
	if params.Query != nil {
		query = strings.TrimSpace(*params.Query)
	}
	status := ""
	if params.Status != nil {
		status = string(*params.Status)
	}
	response := api.FiscalVoucherList{Items: make([]api.FiscalVoucher, 0)}
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM fiscal.vouchers AS voucher
				 WHERE voucher.environment = $1
				   AND ($2 = '' OR voucher.status = $2)
				   AND (
						$3 = ''
						OR voucher.source_type ILIKE '%' || $3 || '%'
						OR voucher.source_id ILIKE '%' || $3 || '%'
						OR coalesce(voucher.authorization_code, '') ILIKE '%' || $3 || '%'
						OR coalesce(voucher.voucher_number::text, '') ILIKE '%' || $3 || '%'
				   )`,
				string(params.Environment), status, query,
			).Scan(&response.Page.Total); err != nil {
				return fmt.Errorf("count fiscal vouchers: %w", err)
			}
			rows, err := tx.Query(ctx, fiscalVoucherSelect+`
				 WHERE voucher.environment = $1
				   AND ($2 = '' OR voucher.status = $2)
				   AND (
						$3 = ''
						OR voucher.source_type ILIKE '%' || $3 || '%'
						OR voucher.source_id ILIKE '%' || $3 || '%'
						OR coalesce(voucher.authorization_code, '') ILIKE '%' || $3 || '%'
						OR coalesce(voucher.voucher_number::text, '') ILIKE '%' || $3 || '%'
				   )
				   AND (
						$4 = ''
						OR (voucher.created_at, voucher.id) < ($4::timestamptz, $5::uuid)
				   )
				 ORDER BY voucher.created_at DESC, voucher.id DESC
				 LIMIT $6`,
				string(params.Environment), status, query,
				cursor.Sort, nullableCursorID(cursor.ID), limit+1,
			)
			if err != nil {
				return fmt.Errorf("list fiscal vouchers: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				item, err := scanFiscalVoucherAPI(rows)
				if err != nil {
					return err
				}
				response.Items = append(response.Items, item)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate fiscal vouchers: %w", err)
			}
			if len(response.Items) > limit {
				last := response.Items[limit-1]
				response.Items = response.Items[:limit]
				response.Page.NextCursor = encodeKeysetCursor(
					last.CreatedAt.UTC().Format(time.RFC3339Nano),
					last.Id.String(),
				)
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) createFiscalVoucher(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateFiscalVoucherParams,
) {
	var input api.FiscalVoucherInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.FiscalVoucher
	if !h.withFiscalService(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			service *fiscal.Service,
			organizationID uuid.UUID,
			actor string,
			tx pgx.Tx,
		) error {
			queueInput, voucherType, err := h.fiscalQueueInput(
				ctx, tx, organizationID, actor, string(params.IdempotencyKey),
				fiscal.OperationInvoice, input, nil,
			)
			if err != nil {
				return err
			}
			result, err := service.Queue(ctx, queueInput)
			if err != nil {
				return err
			}
			response, err = fiscalVoucherFromDomain(result.Voucher, voucherType, nil)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) getFiscalVoucher(w http.ResponseWriter, r *http.Request, voucherID api.VoucherID) {
	var response api.FiscalVoucherDetail
	if !h.withFiscalService(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			_ *fiscal.Service,
			organizationID uuid.UUID,
			_ string,
			tx pgx.Tx,
		) error {
			repository, err := fiscalpg.New(tx)
			if err != nil {
				return err
			}
			voucher, err := repository.Get(ctx, organizationID, voucherID)
			if err != nil {
				return err
			}
			response, err = fiscalVoucherDetailFromDomain(voucher)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) getFiscalVoucherPDF(w http.ResponseWriter, r *http.Request, voucherID api.VoucherID) {
	if h.fiscalObjects == nil {
		writeAPIError(
			w, http.StatusServiceUnavailable, "FISCAL_OBJECT_STORE_UNAVAILABLE",
			"Immutable fiscal object storage is not configured",
		)
		return
	}
	var reference fiscal.ArtifactReference
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			if err := tx.QueryRow(ctx, `
				SELECT artifact_type, content_type, storage_ref, sha256
				  FROM fiscal.voucher_artifacts
				 WHERE voucher_id = $1
				   AND artifact_type = 'pdf'
				 ORDER BY artifact_version DESC
				 LIMIT 1`,
				voucherID,
			).Scan(
				&reference.Kind, &reference.ContentType, &reference.Key, &reference.SHA256,
			); err != nil {
				return err
			}
			return nil
		},
	) {
		return
	}
	object, err := h.fiscalObjects.Get(r.Context(), reference.Key)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "FISCAL_OBJECT_UNAVAILABLE", "Fiscal PDF is unavailable")
		return
	}
	digest := sha256.Sum256(object.Body)
	if object.Key != reference.Key ||
		hex.EncodeToString(digest[:]) != reference.SHA256 ||
		(object.SHA256 != "" && object.SHA256 != reference.SHA256) {
		writeAPIError(w, http.StatusConflict, "FISCAL_ARTIFACT_INTEGRITY", "Fiscal PDF integrity check failed")
		return
	}
	contentType := reference.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/pdf"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="comprobante-%s.pdf"`, voucherID))
	w.Header().Set("ETag", `"`+reference.SHA256+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(object.Body)
}

type fiscalWork func(
	context.Context,
	*fiscal.Service,
	uuid.UUID,
	string,
	pgx.Tx,
) error

func (h *IAMAPI) withFiscalService(
	w http.ResponseWriter,
	r *http.Request,
	permission productiam.Permission,
	work fiscalWork,
) bool {
	return h.withinBusinessTx(
		w,
		r,
		permission,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active fiscal organization: %w", err)
			}
			repository, err := fiscalpg.New(tx)
			if err != nil {
				return err
			}
			return mapFiscalError(work(
				ctx,
				fiscal.NewService(repository),
				organizationID,
				active.UserID,
				tx,
			))
		},
	)
}

func mapFiscalError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fiscal.ErrNotFound):
		return fmt.Errorf("%w: %v", errBusinessNotFound, err)
	case errors.Is(err, fiscal.ErrIdempotencyConflict):
		return fmt.Errorf("%w: %v", errBusinessIdempotency, err)
	case errors.Is(err, fiscal.ErrSourceAlreadyUsed):
		return fmt.Errorf("%w: %v", errBusinessDuplicate, err)
	case errors.Is(err, fiscal.ErrSequenceConflict):
		return fmt.Errorf("%w: %v", errFiscalUncertain, err)
	case errors.Is(err, fiscal.ErrLeaseLost):
		return fmt.Errorf("%w: %v", errBusinessVersionConflict, err)
	default:
		return err
	}
}

type fiscalIssuer struct {
	environment       string
	pointOfSale       int
	legalName         string
	taxAddress        string
	activityStartDate time.Time
	cuit              string
	taxCondition      string
}

func (h *IAMAPI) fiscalQueueInput(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	actor, idempotencyKey string,
	operation fiscal.Operation,
	input api.FiscalVoucherInput,
	associated *fiscal.AssociatedDocumentSnapshot,
) (fiscal.QueueVoucherInput, ar.VoucherType, error) {
	if !input.SourceType.Valid() ||
		!input.Concept.Valid() ||
		!input.ReceiverDocumentType.Valid() ||
		!input.SaleCondition.Valid() ||
		!input.PaymentMethod.Valid() ||
		input.SourceId == uuid.Nil ||
		input.PointOfSaleId == uuid.Nil ||
		len(input.Lines) == 0 ||
		len(input.Lines) > 1000 {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf("%w: invalid fiscal voucher input", errBusinessInvalidRequest)
	}
	onCredit := input.SaleCondition == api.FiscalVoucherInputSaleConditionCredit
	if onCredit {
		if input.PartyId == nil ||
			uuid.UUID(*input.PartyId) == uuid.Nil ||
			input.AccountingDueDate == nil {
			return fiscal.QueueVoucherInput{}, 0, fmt.Errorf(
				"%w: credit sales require a customer and accounting due date",
				errBusinessInvalidRequest,
			)
		}
	} else if input.PartyId != nil || input.AccountingDueDate != nil {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf(
			"%w: cash sales cannot create a customer open item",
			errBusinessInvalidRequest,
		)
	}
	if input.SourceType != api.Sale {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf(
			"%w: outbound ARCA authorization only accepts sale sources; supplier vouchers require registration data",
			errBusinessInvalidRequest,
		)
	}
	issuer, err := loadFiscalIssuer(ctx, tx, organizationID, input.PointOfSaleId)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, err
	}
	if string(input.Environment) != issuer.environment {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf(
			"%w: point of sale does not belong to the selected environment",
			errBusinessInvalidRequest,
		)
	}
	issuerCondition, err := ar.ParseVATCondition(issuer.taxCondition)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	receiverCondition, err := ar.ParseVATCondition(input.ReceiverTaxCondition)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	voucherType, err := ar.VoucherTypeFor(operation, issuerCondition, receiverCondition)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	document, err := h.buildFiscalSnapshot(input, issuer, voucherType, associated)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, err
	}
	snapshot, err := fiscal.NewSnapshot(document)
	if err != nil {
		return fiscal.QueueVoucherInput{}, 0, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	return fiscal.QueueVoucherInput{
		OrganizationID: organizationID,
		IdempotencyKey: idempotencyKey,
		Source: fiscal.SourceReference{
			Kind: string(input.SourceType),
			ID:   input.SourceId,
		},
		Operation:     operation,
		Environment:   issuer.environment,
		PointOfSale:   issuer.pointOfSale,
		AuthorityType: int(voucherType),
		Snapshot:      snapshot,
		Actor:         actor,
	}, voucherType, nil
}

func loadFiscalIssuer(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	pointOfSaleID uuid.UUID,
) (fiscalIssuer, error) {
	var issuer fiscalIssuer
	var address []byte
	err := tx.QueryRow(ctx, `
		SELECT
			point.environment,
			point.code,
			profile.legal_name,
			profile.legal_address,
			profile.activity_start_date,
			settings.cuit::text,
			settings.iva_condition
		  FROM fiscal.points_of_sale AS point
		  JOIN fiscal.profiles AS profile
		    ON profile.org_id = point.org_id
		  JOIN fiscal_ar.settings AS settings
		    ON settings.org_id = point.org_id
		   AND settings.environment = point.environment
		 WHERE point.org_id = $1
		   AND point.id = $2
		   AND point.country_code = 'AR'
		   AND point.enabled
		   AND settings.enabled
		   AND EXISTS (
				SELECT 1
				  FROM fiscal.certificates AS certificate
				 WHERE certificate.org_id = point.org_id
				   AND certificate.environment = point.environment
				   AND certificate.status = 'active'
				   AND certificate.valid_from <= now()
				   AND certificate.valid_until > now()
		   )`,
		organizationID, pointOfSaleID,
	).Scan(
		&issuer.environment,
		&issuer.pointOfSale,
		&issuer.legalName,
		&address,
		&issuer.activityStartDate,
		&issuer.cuit,
		&issuer.taxCondition,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiscalIssuer{}, fmt.Errorf("%w: enabled fiscal point of sale/settings", errBusinessNotFound)
	}
	if err != nil {
		return fiscalIssuer{}, fmt.Errorf("load fiscal issuer: %w", err)
	}
	issuer.taxAddress = fiscalAddress(address)
	return issuer, nil
}

func (h *IAMAPI) buildFiscalSnapshot(
	input api.FiscalVoucherInput,
	issuer fiscalIssuer,
	voucherType ar.VoucherType,
	associated *fiscal.AssociatedDocumentSnapshot,
) (fiscal.FiscalSnapshot, error) {
	currency, err := ar.CurrencyCode(input.Currency)
	if err != nil {
		return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	rate, err := fiscal.ParseDecimal(input.ExchangeRate)
	if err != nil || rate.Cmp(fiscal.Decimal{}) <= 0 {
		return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: invalid exchange rate", errBusinessInvalidRequest)
	}
	if currency == ar.CurrencyPES && !rate.Equal(fiscal.NewDecimalFromInt(1)) {
		return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: ARS exchange rate must equal one", errBusinessInvalidRequest)
	}
	lines, taxes, totals, err := buildFiscalLines(input.Lines, voucherType)
	if err != nil {
		return fiscal.FiscalSnapshot{}, err
	}
	totals.Functional, err = totals.Total.
		Mul(rate).
		Quantize(2, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: invalid functional total", errBusinessInvalidRequest)
	}
	issueDate := h.now().UTC().Format("2006-01-02")
	issueDay, err := time.Parse("2006-01-02", issueDate)
	if err != nil {
		return fiscal.FiscalSnapshot{}, fmt.Errorf(
			"parse fiscal issue date: %w",
			err,
		)
	}
	documentType, documentNumber, err := fiscalReceiverIdentity(
		input.ReceiverDocumentType, input.ReceiverDocumentNumber,
	)
	if err != nil {
		return fiscal.FiscalSnapshot{}, err
	}
	receiverName := "Consumidor final"
	if input.ReceiverName != nil && strings.TrimSpace(*input.ReceiverName) != "" {
		receiverName = strings.TrimSpace(*input.ReceiverName)
	} else if input.ReceiverDocumentType != api.FiscalVoucherInputReceiverDocumentTypeCONSUMERFINAL {
		return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: identified receivers require a name", errBusinessInvalidRequest)
	}
	document := fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   issueDate,
		Issuer: fiscal.PartySnapshot{
			Name:             issuer.legalName,
			TaxID:            issuer.cuit,
			TaxCondition:     issuer.taxCondition,
			Address:          issuer.taxAddress,
			ActivityStartDay: issuer.activityStartDate.Format("2006-01-02"),
		},
		Receiver: fiscal.PartySnapshot{
			Name:           receiverName,
			TaxID:          documentNumber,
			TaxCondition:   input.ReceiverTaxCondition,
			DocumentType:   strconv.Itoa(int(documentType)),
			DocumentNumber: documentNumber,
		},
		Currency: fiscal.CurrencySnapshot{
			Code:       currency,
			Rate:       rate,
			RateDate:   issueDate,
			RateSource: nonEmptyString(input.ExchangeRateSource, "user"),
		},
		Lines:              lines,
		Taxes:              taxes,
		Totals:             totals,
		AssociatedDocument: associated,
		Metadata: map[string]string{
			"concept":                            string(input.Concept),
			"voucher_type":                       strconv.Itoa(int(voucherType)),
			"point_of_sale":                      strconv.Itoa(issuer.pointOfSale),
			"accounting.on_credit":               strconv.FormatBool(input.SaleCondition == api.FiscalVoucherInputSaleConditionCredit),
			"accounting.payment_method":          string(input.PaymentMethod),
			"accounting.original_payment_method": string(input.PaymentMethod),
		},
	}
	if input.PartyId != nil {
		document.Metadata["accounting.party_id"] =
			uuid.UUID(*input.PartyId).String()
	}
	if input.AccountingDueDate != nil {
		if input.AccountingDueDate.Time.Before(issueDay) {
			return fiscal.FiscalSnapshot{}, fmt.Errorf(
				"%w: accounting due date cannot precede issue date",
				errBusinessInvalidRequest,
			)
		}
		document.Metadata["accounting.due_date"] =
			input.AccountingDueDate.Time.Format("2006-01-02")
	}
	switch input.Concept {
	case api.Products:
		if input.ServiceFrom != nil || input.ServiceTo != nil || input.PaymentDueDate != nil {
			return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: products cannot include service dates", errBusinessInvalidRequest)
		}
	case api.Services, api.Mixed:
		if input.ServiceFrom == nil || input.ServiceTo == nil || input.PaymentDueDate == nil {
			return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: services require service and due dates", errBusinessInvalidRequest)
		}
		if input.ServiceFrom.Time.After(input.ServiceTo.Time) {
			return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: service start date cannot be after service end date", errBusinessInvalidRequest)
		}
		if input.PaymentDueDate.Time.Before(issueDay) {
			return fiscal.FiscalSnapshot{}, fmt.Errorf("%w: payment due date cannot precede issue date", errBusinessInvalidRequest)
		}
		document.ServiceFrom = input.ServiceFrom.Time.Format("2006-01-02")
		document.ServiceTo = input.ServiceTo.Time.Format("2006-01-02")
		document.PaymentDue = input.PaymentDueDate.Time.Format("2006-01-02")
	}
	return document, nil
}

func buildFiscalLines(
	input []api.FiscalVoucherLineInput,
	voucherType ar.VoucherType,
) ([]fiscal.FiscalLineSnapshot, []fiscal.TaxSnapshot, fiscal.FiscalTotalsSnapshot, error) {
	lines := make([]fiscal.FiscalLineSnapshot, 0, len(input))
	taxes := make([]fiscal.TaxSnapshot, 0)
	amounts := make([]ar.TaxableAmount, 0, len(input))
	tributes := make([]ar.Tribute, 0)
	lineVATTotal := fiscal.Decimal{}
	withholdings := fiscal.Decimal{}
	perceptions := fiscal.Decimal{}
	for index, item := range input {
		if strings.TrimSpace(item.Description) == "" || len(strings.TrimSpace(item.Description)) > 500 {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid description on line %d", errBusinessInvalidRequest, index+1)
		}
		quantity, err := fiscal.ParseDecimal(item.Quantity)
		if err != nil || quantity.Cmp(fiscal.Decimal{}) <= 0 {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid quantity on line %d", errBusinessInvalidRequest, index+1)
		}
		unitPrice, err := fiscal.ParseDecimal(item.UnitPrice)
		if err != nil || unitPrice.IsNegative() {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid unit price on line %d", errBusinessInvalidRequest, index+1)
		}
		subtotal, err := fiscal.ParseDecimal(item.Subtotal)
		if err != nil || subtotal.IsNegative() {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid subtotal on line %d", errBusinessInvalidRequest, index+1)
		}
		subtotal, err = subtotal.Quantize(2, fiscal.RoundHalfAwayFromZero)
		if err != nil {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, err
		}
		expectedSubtotal, err := quantity.Mul(unitPrice).Quantize(2, fiscal.RoundHalfAwayFromZero)
		if err != nil || !expectedSubtotal.Equal(subtotal) {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: line %d subtotal does not equal quantity times unit price", errBusinessInvalidRequest, index+1)
		}
		costConfirmed := item.CostConfirmed != nil && *item.CostConfirmed
		costAmount := fiscal.Decimal{}
		if item.CostAmount != nil {
			costAmount, err = fiscal.ParseDecimal(*item.CostAmount)
			if err != nil || costAmount.IsNegative() {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf(
					"%w: invalid confirmed cost on line %d",
					errBusinessInvalidRequest,
					index+1,
				)
			}
			quantized, quantizeErr := costAmount.Quantize(
				6,
				fiscal.RoundHalfAwayFromZero,
			)
			if quantizeErr != nil || !quantized.Equal(costAmount) {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf(
					"%w: line %d cost has more than six decimal places",
					errBusinessInvalidRequest,
					index+1,
				)
			}
		}
		if costConfirmed != (item.CostAmount != nil) {
			return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf(
				"%w: line %d cost amount and confirmation must be supplied together",
				errBusinessInvalidRequest,
				index+1,
			)
		}
		category := ar.Taxable
		rate := fiscal.Decimal{}
		vatAmount := fiscal.Decimal{}
		exemptAmount := fiscal.Decimal{}
		untaxedAmount := fiscal.Decimal{}
		taxCode := ""
		categorySet := false
		for taxIndex, component := range item.Taxes {
			if !component.Kind.Valid() {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid tax component", errBusinessInvalidRequest)
			}
			base, parseErr := fiscal.ParseDecimal(component.TaxableBase)
			if parseErr != nil || base.IsNegative() {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid taxable base", errBusinessInvalidRequest)
			}
			componentRate, parseErr := fiscal.ParseDecimal(component.Rate)
			if parseErr != nil || componentRate.IsNegative() {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid tax rate", errBusinessInvalidRequest)
			}
			amount, parseErr := fiscal.ParseDecimal(component.Amount)
			if parseErr != nil || amount.IsNegative() {
				return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: invalid tax amount", errBusinessInvalidRequest)
			}
			switch component.Kind {
			case api.FiscalTaxComponentKindVat:
				if categorySet {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: line has conflicting tax treatments", errBusinessInvalidRequest)
				}
				categorySet = true
				category, rate, vatAmount = ar.Taxable, componentRate, amount
				taxCode = "IVA" + componentRate.String()
				if voucherType.IsTypeC() && (!amount.IsZero() || !componentRate.IsZero()) {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: type C vouchers cannot discriminate VAT", errBusinessInvalidRequest)
				}
				if !base.Equal(subtotal) {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: VAT base must equal line subtotal", errBusinessInvalidRequest)
				}
				expectedVAT, calculationErr := subtotal.
					Mul(componentRate).
					Quo(fiscal.NewDecimalFromInt(100), 2, fiscal.RoundHalfAwayFromZero)
				if calculationErr != nil || !expectedVAT.Equal(amount) {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: VAT amount does not match base and rate", errBusinessInvalidRequest)
				}
			case api.FiscalTaxComponentKindExempt:
				if voucherType.IsTypeC() {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: type C tax treatment is included in price and must not be discriminated", errBusinessInvalidRequest)
				}
				if categorySet {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: line has conflicting tax treatments", errBusinessInvalidRequest)
				}
				if !base.Equal(subtotal) || !componentRate.IsZero() || !amount.IsZero() {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: exempt treatment requires subtotal base and zero rate/amount", errBusinessInvalidRequest)
				}
				categorySet = true
				category, exemptAmount = ar.Exempt, subtotal
			case api.FiscalTaxComponentKindNonTaxed:
				if voucherType.IsTypeC() {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: type C tax treatment is included in price and must not be discriminated", errBusinessInvalidRequest)
				}
				if categorySet {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: line has conflicting tax treatments", errBusinessInvalidRequest)
				}
				if !base.Equal(subtotal) || !componentRate.IsZero() || !amount.IsZero() {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: non-taxed treatment requires subtotal base and zero rate/amount", errBusinessInvalidRequest)
				}
				categorySet = true
				category, untaxedAmount = ar.Untaxed, subtotal
			default:
				if component.AuthorityCode == nil ||
					*component.AuthorityCode <= 0 ||
					component.Description == nil ||
					strings.TrimSpace(*component.Description) == "" {
					return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf(
						"%w: non-VAT tax %d requires ARCA code and description",
						errBusinessInvalidRequest,
						taxIndex+1,
					)
				}
				description := strings.TrimSpace(*component.Description)
				tributes = append(tributes, ar.Tribute{
					ID: *component.AuthorityCode, Description: description,
					BaseAmount: base, Rate: componentRate, Amount: amount,
				})
				taxes = append(taxes, fiscal.TaxSnapshot{
					Code: strconv.Itoa(*component.AuthorityCode), Description: description,
					BaseAmount: base, Rate: componentRate, Amount: amount,
				})
				if component.Kind == api.FiscalTaxComponentKindWithholding {
					withholdings = withholdings.Add(amount)
				}
				if component.Kind == api.FiscalTaxComponentKindPerception {
					perceptions = perceptions.Add(amount)
				}
			}
		}
		amounts = append(amounts, ar.TaxableAmount{
			Category: category, Amount: subtotal, Rate: rate,
		})
		lineVATTotal = lineVATTotal.Add(vatAmount)
		lines = append(lines, fiscal.FiscalLineSnapshot{
			Position: index + 1, Description: strings.TrimSpace(item.Description),
			Quantity: quantity, UnitPrice: unitPrice, NetAmount: subtotal,
			TaxCode: taxCode, TaxRate: rate, TaxAmount: vatAmount,
			ExemptAmount: exemptAmount, UntaxedAmount: untaxedAmount,
			TotalAmount: subtotal.Add(vatAmount),
			CostAmount:  costAmount, CostConfirmed: costConfirmed,
		})
	}
	calculated, err := ar.CalculateTotals(voucherType, amounts, tributes)
	if err != nil {
		return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	if !calculated.VAT.Equal(lineVATTotal) {
		return nil, nil, fiscal.FiscalTotalsSnapshot{}, fmt.Errorf("%w: VAT amounts do not reconcile with rates", errBusinessInvalidRequest)
	}
	return lines, taxes, fiscal.FiscalTotalsSnapshot{
		NetTaxed: calculated.NetTaxed, NetUntaxed: calculated.NetUntaxed,
		Exempt: calculated.Exempt, VAT: calculated.VAT,
		OtherTaxes: calculated.Tributes, Total: calculated.Total,
		Functional: calculated.Total, Withholdings: withholdings,
		Perceptions: perceptions,
	}, nil
}

func fiscalReceiverIdentity(
	documentType api.FiscalVoucherInputReceiverDocumentType,
	raw string,
) (ar.DocumentType, string, error) {
	var kind ar.DocumentType
	switch documentType {
	case api.FiscalVoucherInputReceiverDocumentTypeCUIT:
		kind = ar.DocumentCUIT
	case api.FiscalVoucherInputReceiverDocumentTypeCUIL:
		kind = ar.DocumentCUIL
	case api.FiscalVoucherInputReceiverDocumentTypeDNI:
		kind = ar.DocumentDNI
	case api.FiscalVoucherInputReceiverDocumentTypeCONSUMERFINAL:
		kind = ar.DocumentConsumerFinal
	default:
		return 0, "", fmt.Errorf("%w: unsupported receiver document", errBusinessInvalidRequest)
	}
	document, err := ar.NewReceiverDocument(kind, raw)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	return document.Type, document.Number, nil
}

func nonEmptyString(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return strings.TrimSpace(*value)
	}
	return fallback
}

func (h *IAMAPI) createFiscalAdjustment(
	w http.ResponseWriter,
	r *http.Request,
	operation fiscal.Operation,
	idempotencyKey string,
) {
	var input api.FiscalAdjustmentInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" ||
		len(strings.TrimSpace(input.Reason)) > 500 ||
		input.AssociatedVoucherId == uuid.Nil ||
		len(input.Lines) == 0 ||
		len(input.Lines) > 1000 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Adjustment reason and lines are required")
		return
	}
	var response api.FiscalVoucher
	if !h.withFiscalService(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			service *fiscal.Service,
			organizationID uuid.UUID,
			actor string,
			tx pgx.Tx,
		) error {
			repository, err := fiscalpg.New(tx)
			if err != nil {
				return err
			}
			original, err := repository.Get(ctx, organizationID, input.AssociatedVoucherId)
			if err != nil {
				return err
			}
			if original.Status != fiscal.StatusAuthorized ||
				original.Operation != fiscal.OperationInvoice ||
				original.Authorization == nil {
				return fmt.Errorf("%w: only an authorized invoice can be adjusted", errBusinessInvalidTransition)
			}
			noteType, err := ar.NoteTypeFor(ar.VoucherType(original.AuthorityType), operation)
			if err != nil {
				return fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
			}
			document, err := original.Snapshot.Document()
			if err != nil {
				return err
			}
			lines, taxes, totals, err := buildFiscalLines(input.Lines, noteType)
			if err != nil {
				return err
			}
			totals.Functional, err = totals.Total.
				Mul(document.Currency.Rate).
				Quantize(2, fiscal.RoundHalfAwayFromZero)
			if err != nil {
				return err
			}
			document.Lines = lines
			document.Taxes = taxes
			document.Totals = totals
			document.IssueDate = h.now().UTC().Format("2006-01-02")
			document.AssociatedDocument = &fiscal.AssociatedDocumentSnapshot{
				VoucherID:   original.ID.String(),
				Type:        original.AuthorityType,
				PointOfSale: original.PointOfSale,
				Number:      original.Number,
				IssueDate:   fiscalVoucherIssueDate(original),
				IssuerTaxID: document.Issuer.TaxID,
			}
			if document.Metadata == nil {
				document.Metadata = make(map[string]string)
			}
			document.Metadata["adjustment_reason"] = strings.TrimSpace(input.Reason)
			document.Metadata["adjustment_operation"] = string(operation)
			snapshot, err := fiscal.NewSnapshot(document)
			if err != nil {
				return fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
			}
			sourceID := uuid.NewSHA1(
				uuid.NameSpaceOID,
				[]byte(organizationID.String()+":"+original.ID.String()+":"+string(operation)+":"+idempotencyKey),
			)
			result, err := service.Queue(ctx, fiscal.QueueVoucherInput{
				OrganizationID: organizationID,
				IdempotencyKey: idempotencyKey,
				Source: fiscal.SourceReference{
					Kind: "adjustment", ID: sourceID,
				},
				Operation: operation, Environment: original.Environment,
				PointOfSale: original.PointOfSale, AuthorityType: int(noteType),
				Snapshot: snapshot, Actor: actor,
			})
			if err != nil {
				return err
			}
			associatedID := original.ID
			response, err = fiscalVoucherFromDomain(result.Voucher, noteType, &associatedID)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func fiscalVoucherIssueDate(voucher fiscal.Voucher) string {
	document, err := voucher.Snapshot.Document()
	if err != nil {
		return ""
	}
	return document.IssueDate
}

const fiscalVoucherSelect = `
	SELECT
		voucher.id,
		voucher.environment,
		voucher.status,
		voucher.voucher_type,
		voucher.source_type,
		voucher.source_id,
		voucher.concept,
		point.code,
		voucher.voucher_number,
		voucher.currency_code,
		voucher.total_amount::text,
		voucher.authorization_code,
		voucher.authorization_expires_at,
		association.associated_voucher_id,
		coalesce(voucher.arca_result, ''),
		voucher.created_at,
		coalesce(
			voucher.authorized_at,
			voucher.rejected_at,
			voucher.uncertain_at,
			voucher.created_at
		)
	  FROM fiscal.vouchers AS voucher
	  JOIN fiscal.points_of_sale AS point
	    ON point.org_id = voucher.org_id
	   AND point.id = voucher.point_of_sale_id
	  LEFT JOIN fiscal.voucher_associations AS association
	    ON association.org_id = voucher.org_id
	   AND association.voucher_id = voucher.id`

func scanFiscalVoucherAPI(scanner interface{ Scan(...any) error }) (api.FiscalVoucher, error) {
	var (
		response          api.FiscalVoucher
		status            string
		voucherType       int
		sourceID          string
		concept           string
		pointOfSale       int
		voucherNumber     *int64
		authorizationCode *string
		expiresAt         *time.Time
		associatedID      *uuid.UUID
		authorizationRaw  string
	)
	if err := scanner.Scan(
		&response.Id,
		&response.Environment,
		&status,
		&voucherType,
		&response.SourceType,
		&sourceID,
		&concept,
		&pointOfSale,
		&voucherNumber,
		&response.Currency,
		&response.Total,
		&authorizationCode,
		&expiresAt,
		&associatedID,
		&authorizationRaw,
		&response.CreatedAt,
		&response.UpdatedAt,
	); err != nil {
		return api.FiscalVoucher{}, fmt.Errorf("scan fiscal voucher: %w", err)
	}
	parsedSourceID, err := uuid.Parse(sourceID)
	if err != nil {
		return api.FiscalVoucher{}, fmt.Errorf("parse fiscal source id: %w", err)
	}
	kind, err := apiVoucherKind(ar.VoucherType(voucherType))
	if err != nil {
		return api.FiscalVoucher{}, err
	}
	response.SourceId = parsedSourceID
	response.State = api.FiscalAuthorizationState(status)
	response.Concept = api.FiscalConcept(concept)
	response.PointOfSale = &pointOfSale
	response.VoucherNumber = voucherNumber
	response.Cae = authorizationCode
	response.AssociatedVoucherId = associatedID
	response.Kind = &kind
	response.Currency = apiCurrencyCode(response.Currency)
	if expiresAt != nil {
		date := openapi_types.Date{Time: *expiresAt}
		response.CaeExpiresAt = &date
	}
	if authorizationRaw != "" {
		var authorization fiscal.Authorization
		if err := json.Unmarshal([]byte(authorizationRaw), &authorization); err != nil {
			return api.FiscalVoucher{}, fmt.Errorf("parse fiscal authorization result: %w", err)
		}
		observations := make([]string, 0, len(authorization.Observations)+len(authorization.Errors))
		for _, note := range authorization.Observations {
			observations = append(observations, strings.TrimSpace(note.Code+" "+note.Message))
		}
		for _, note := range authorization.Errors {
			observations = append(observations, strings.TrimSpace(note.Code+" "+note.Message))
		}
		if len(observations) > 0 {
			response.Observations = &observations
		}
	}
	return response, nil
}

func fiscalVoucherFromDomain(
	voucher fiscal.Voucher,
	voucherType ar.VoucherType,
	associatedID *uuid.UUID,
) (api.FiscalVoucher, error) {
	document, err := voucher.Snapshot.Document()
	if err != nil {
		return api.FiscalVoucher{}, err
	}
	kind, err := apiVoucherKind(voucherType)
	if err != nil {
		return api.FiscalVoucher{}, err
	}
	concept := api.FiscalConcept(document.Metadata["concept"])
	if !concept.Valid() {
		concept = api.Products
		if document.ServiceFrom != "" {
			concept = api.Services
		}
	}
	pointOfSale := voucher.PointOfSale
	response := api.FiscalVoucher{
		Id: voucher.ID, State: api.FiscalAuthorizationState(voucher.Status),
		Kind: &kind, SourceType: voucher.Source.Kind, SourceId: voucher.Source.ID,
		Concept: concept, PointOfSale: &pointOfSale,
		Environment: api.FiscalEnvironment(voucher.Environment),
		Currency:    apiCurrencyCode(document.Currency.Code),
		Total:       document.Totals.Total.String(), AssociatedVoucherId: associatedID,
		CreatedAt: voucher.CreatedAt, UpdatedAt: voucher.UpdatedAt,
	}
	if voucher.Number > 0 {
		number := voucher.Number
		response.VoucherNumber = &number
	}
	if voucher.Authorization != nil {
		if voucher.Authorization.Code != "" {
			code := voucher.Authorization.Code
			response.Cae = &code
		}
		if voucher.Authorization.ExpiresOn != "" {
			expiresAt, err := time.Parse("2006-01-02", voucher.Authorization.ExpiresOn)
			if err != nil {
				return api.FiscalVoucher{}, err
			}
			date := openapi_types.Date{Time: expiresAt}
			response.CaeExpiresAt = &date
		}
		observations := make(
			[]string,
			0,
			len(voucher.Authorization.Observations)+len(voucher.Authorization.Errors),
		)
		for _, note := range voucher.Authorization.Observations {
			observations = append(
				observations,
				strings.TrimSpace(note.Code+" "+note.Message),
			)
		}
		for _, note := range voucher.Authorization.Errors {
			observations = append(
				observations,
				strings.TrimSpace(note.Code+" "+note.Message),
			)
		}
		if len(observations) > 0 {
			response.Observations = &observations
		}
	}
	return response, nil
}

func fiscalVoucherDetailFromDomain(voucher fiscal.Voucher) (api.FiscalVoucherDetail, error) {
	summary, err := fiscalVoucherFromDomain(
		voucher,
		ar.VoucherType(voucher.AuthorityType),
		associatedVoucherID(voucher),
	)
	if err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	document, err := voucher.Snapshot.Document()
	if err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	issueDate, err := parseFiscalAPIDate(document.IssueDate)
	if err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	documentType, err := fiscalDocumentTypeName(document.Receiver.DocumentType)
	if err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	lines := make([]api.FiscalVoucherSnapshotLine, 0, len(document.Lines))
	for _, line := range document.Lines {
		treatment := api.FiscalVoucherSnapshotLineTaxTreatmentTaxable
		subtotal := line.NetAmount
		switch {
		case !line.ExemptAmount.IsZero():
			treatment = api.FiscalVoucherSnapshotLineTaxTreatmentExempt
			subtotal = line.ExemptAmount
		case !line.UntaxedAmount.IsZero():
			treatment = api.FiscalVoucherSnapshotLineTaxTreatmentNonTaxed
			subtotal = line.UntaxedAmount
		}
		lines = append(lines, api.FiscalVoucherSnapshotLine{
			Position: line.Position, Description: line.Description,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			TaxTreatment: treatment, Subtotal: subtotal.String(),
			VatRate: line.TaxRate.String(), VatAmount: line.TaxAmount.String(),
			Total: line.TotalAmount.String(),
		})
	}
	detail := api.FiscalVoucherDetail{
		AssociatedVoucherId: summary.AssociatedVoucherId,
		Cae:                 summary.Cae, CaeExpiresAt: summary.CaeExpiresAt,
		Concept: summary.Concept, CreatedAt: summary.CreatedAt,
		Currency: summary.Currency, Environment: summary.Environment,
		ExchangeRate: document.Currency.Rate.String(),
		Id:           summary.Id, IssueDate: issueDate, Kind: summary.Kind, Lines: lines,
		Observations: summary.Observations, PointOfSale: summary.PointOfSale,
		ReceiverDocumentNumber: document.Receiver.DocumentNumber,
		ReceiverDocumentType:   documentType,
		ReceiverName:           document.Receiver.Name,
		ReceiverTaxCondition:   document.Receiver.TaxCondition,
		SourceId:               summary.SourceId, SourceType: summary.SourceType,
		State: summary.State, Total: summary.Total, UpdatedAt: summary.UpdatedAt,
		VoucherNumber: summary.VoucherNumber,
	}
	if document.Currency.RateSource != "" {
		detail.ExchangeRateSource = &document.Currency.RateSource
	}
	if detail.ServiceFrom, err = optionalFiscalAPIDate(document.ServiceFrom); err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	if detail.ServiceTo, err = optionalFiscalAPIDate(document.ServiceTo); err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	if detail.PaymentDueDate, err = optionalFiscalAPIDate(document.PaymentDue); err != nil {
		return api.FiscalVoucherDetail{}, err
	}
	return detail, nil
}

func associatedVoucherID(voucher fiscal.Voucher) *uuid.UUID {
	document, err := voucher.Snapshot.Document()
	if err != nil || document.AssociatedDocument == nil {
		return nil
	}
	id, err := uuid.Parse(document.AssociatedDocument.VoucherID)
	if err != nil {
		return nil
	}
	return &id
}

func parseFiscalAPIDate(value string) (openapi_types.Date, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return openapi_types.Date{}, fmt.Errorf("parse fiscal snapshot date: %w", err)
	}
	return openapi_types.Date{Time: parsed}, nil
}

func optionalFiscalAPIDate(value string) (*openapi_types.Date, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseFiscalAPIDate(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func fiscalDocumentTypeName(value string) (api.FiscalVoucherDetailReceiverDocumentType, error) {
	switch value {
	case strconv.Itoa(int(ar.DocumentCUIT)):
		return api.FiscalVoucherDetailReceiverDocumentTypeCUIT, nil
	case strconv.Itoa(int(ar.DocumentCUIL)):
		return api.FiscalVoucherDetailReceiverDocumentTypeCUIL, nil
	case strconv.Itoa(int(ar.DocumentDNI)):
		return api.FiscalVoucherDetailReceiverDocumentTypeDNI, nil
	case strconv.Itoa(int(ar.DocumentConsumerFinal)):
		return api.FiscalVoucherDetailReceiverDocumentTypeCONSUMERFINAL, nil
	default:
		return "", fmt.Errorf("unsupported fiscal receiver document type %q", value)
	}
}

func apiVoucherKind(voucherType ar.VoucherType) (api.FiscalVoucherKind, error) {
	switch voucherType {
	case ar.InvoiceA:
		return api.InvoiceA, nil
	case ar.InvoiceB:
		return api.InvoiceB, nil
	case ar.InvoiceC:
		return api.InvoiceC, nil
	case ar.CreditNoteA:
		return api.CreditNoteA, nil
	case ar.CreditNoteB:
		return api.CreditNoteB, nil
	case ar.CreditNoteC:
		return api.CreditNoteC, nil
	case ar.DebitNoteA:
		return api.DebitNoteA, nil
	case ar.DebitNoteB:
		return api.DebitNoteB, nil
	case ar.DebitNoteC:
		return api.DebitNoteC, nil
	default:
		return "", fmt.Errorf("unsupported fiscal voucher type %d", voucherType)
	}
}

func apiCurrencyCode(raw string) string {
	switch raw {
	case ar.CurrencyPES:
		return "ARS"
	case ar.CurrencyDOL:
		return "USD"
	case ar.CurrencyEUR:
		return "EUR"
	default:
		return raw
	}
}

type ivaTotals struct {
	salesNet, outputVAT, purchasesNet, inputVAT fiscal.Decimal
	withholdings, perceptions                   fiscal.Decimal
}

func fiscalPeriod(raw string) (string, time.Time, time.Time, error) {
	period := strings.TrimSpace(raw)
	first, err := time.Parse("2006-01", period)
	if err != nil {
		return "", time.Time{}, time.Time{}, errors.New("IVA Simple period must use YYYY-MM")
	}
	return period, first.UTC(), first.AddDate(0, 1, 0).UTC(), nil
}

func loadIVARecords(
	ctx context.Context,
	tx pgx.Tx,
	firstDay, nextMonth time.Time,
	environment string,
) ([]ar.IVARecord, ivaTotals, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			voucher.source_type,
			voucher.voucher_type,
			point.code,
			voucher.voucher_number,
			snapshot.canonical_json,
			snapshot.snapshot_sha256
		  FROM fiscal.vouchers AS voucher
		  JOIN fiscal.points_of_sale AS point
		    ON point.org_id = voucher.org_id
		   AND point.id = voucher.point_of_sale_id
		  JOIN fiscal.voucher_snapshots AS snapshot
		    ON snapshot.org_id = voucher.org_id
		   AND snapshot.voucher_id = voucher.id
		 WHERE voucher.status = 'authorized'
		   AND voucher.issue_date >= $1
		   AND voucher.issue_date < $2
		   AND voucher.environment = $3
		 ORDER BY voucher.issue_date, point.code, voucher.voucher_number, voucher.id`,
		firstDay, nextMonth, environment,
	)
	if err != nil {
		return nil, ivaTotals{}, fmt.Errorf("query IVA Simple vouchers: %w", err)
	}
	defer rows.Close()
	records := make([]ar.IVARecord, 0)
	var totals ivaTotals
	for rows.Next() {
		var (
			sourceType  string
			voucherType int
			pointOfSale int
			number      int64
			canonical   string
			hash        string
		)
		if err := rows.Scan(
			&sourceType, &voucherType, &pointOfSale, &number, &canonical, &hash,
		); err != nil {
			return nil, ivaTotals{}, fmt.Errorf("scan IVA Simple voucher: %w", err)
		}
		snapshot, err := fiscal.ParseSnapshot([]byte(canonical), hash)
		if err != nil {
			return nil, ivaTotals{}, fmt.Errorf("restore IVA Simple snapshot: %w", err)
		}
		document, err := snapshot.Document()
		if err != nil {
			return nil, ivaTotals{}, err
		}
		documentType, err := strconv.Atoi(document.Receiver.DocumentType)
		if err != nil {
			return nil, ivaTotals{}, fmt.Errorf("parse IVA receiver document type: %w", err)
		}
		receiver, err := ar.NewReceiverDocument(
			ar.DocumentType(documentType),
			nonEmptyRaw(document.Receiver.DocumentNumber, document.Receiver.TaxID),
		)
		if err != nil {
			return nil, ivaTotals{}, fmt.Errorf("validate IVA receiver document: %w", err)
		}
		vatByRate := make(map[string]ar.VATBreakdown)
		order := make([]string, 0)
		for _, line := range document.Lines {
			if line.TaxAmount.IsZero() {
				continue
			}
			id, valid := ar.VATIDForRate(line.TaxRate)
			if !valid {
				return nil, ivaTotals{}, fmt.Errorf("unsupported IVA rate %s", line.TaxRate)
			}
			key := line.TaxRate.String()
			entry, found := vatByRate[key]
			if !found {
				entry.ID, entry.Rate = id, line.TaxRate
				order = append(order, key)
			}
			entry.BaseAmount = entry.BaseAmount.Add(line.NetAmount)
			entry.Amount = entry.Amount.Add(line.TaxAmount)
			vatByRate[key] = entry
		}
		vatLines := make([]ar.VATBreakdown, 0, len(order))
		for _, key := range order {
			vatLines = append(vatLines, vatByRate[key])
		}
		direction := ar.IVASale
		if sourceType == "purchase" {
			direction = ar.IVAPurchase
		}
		record := ar.IVARecord{
			Direction: direction, Authorized: true,
			IssueDate: document.IssueDate, VoucherType: ar.VoucherType(voucherType),
			PointOfSale: pointOfSale, Number: number, NumberTo: number,
			CounterpartyDocument: receiver, CounterpartyName: document.Receiver.Name,
			Currency: document.Currency.Code, ExchangeRate: document.Currency.Rate,
			Total: document.Totals.Total, Untaxed: document.Totals.NetUntaxed,
			Exempt: document.Totals.Exempt, VAT: document.Totals.VAT,
			OtherTaxes:          document.Totals.OtherTaxes,
			ComputableVATCredit: document.Totals.VAT,
			VATLines:            vatLines, PaymentDue: document.PaymentDue,
		}
		for _, tax := range document.Taxes {
			switch tax.Code {
			case string(api.FiscalTaxComponentKindWithholding):
				record.NationalPerceptions = record.NationalPerceptions.Add(tax.Amount)
			case string(api.FiscalTaxComponentKindPerception):
				record.VATPerceptions = record.VATPerceptions.Add(tax.Amount)
			}
		}
		records = append(records, record)
		sign := purchaseVoucherSign(voucherType)
		if direction == ar.IVASale {
			totals.salesNet = totals.salesNet.
				Add(document.Totals.NetTaxed.Mul(sign)).
				Add(document.Totals.NetUntaxed.Mul(sign)).
				Add(document.Totals.Exempt.Mul(sign))
			totals.outputVAT = totals.outputVAT.Add(document.Totals.VAT.Mul(sign))
		} else {
			totals.purchasesNet = totals.purchasesNet.
				Add(document.Totals.NetTaxed.Mul(sign)).
				Add(document.Totals.NetUntaxed.Mul(sign)).
				Add(document.Totals.Exempt.Mul(sign))
			totals.inputVAT = totals.inputVAT.Add(document.Totals.VAT.Mul(sign))
		}
		totals.withholdings = totals.withholdings.Add(document.Totals.Withholdings.Mul(sign))
		totals.perceptions = totals.perceptions.Add(document.Totals.Perceptions.Mul(sign))
	}
	if err := rows.Err(); err != nil {
		return nil, ivaTotals{}, fmt.Errorf("iterate IVA Simple vouchers: %w", err)
	}
	purchaseRecords, purchaseTotals, err := loadIVAPurchaseRecords(
		ctx, tx, firstDay, nextMonth, environment,
	)
	if err != nil {
		return nil, ivaTotals{}, err
	}
	records = append(records, purchaseRecords...)
	totals.purchasesNet = totals.purchasesNet.Add(purchaseTotals.purchasesNet)
	totals.inputVAT = totals.inputVAT.Add(purchaseTotals.inputVAT)
	totals.withholdings = totals.withholdings.Add(purchaseTotals.withholdings)
	totals.perceptions = totals.perceptions.Add(purchaseTotals.perceptions)
	return records, totals, nil
}

func fiscalRegistryBundle(firstName string, first []byte, secondName string, second []byte) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range []struct {
		name string
		body []byte
	}{
		{name: firstName, body: first},
		{name: secondName, body: second},
	} {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create IVA Simple bundle: %w", err)
		}
		if _, err := entry.Write(file.body); err != nil {
			return nil, fmt.Errorf("write IVA Simple bundle: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close IVA Simple bundle: %w", err)
	}
	return output.Bytes(), nil
}

func nonEmptyRaw(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

func privateKeyPublicKey(raw []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("private key PEM is invalid")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch typed := key.(type) {
		case *rsa.PrivateKey:
			return &typed.PublicKey, nil
		case *ecdsa.PrivateKey:
			return &typed.PublicKey, nil
		default:
			return nil, errors.New("private key type is unsupported")
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return &key.PublicKey, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return &key.PublicKey, nil
	}
	return nil, errors.New("private key PEM is unsupported")
}

func publicKeysEqual(left, right crypto.PublicKey) bool {
	leftRaw, leftErr := x509.MarshalPKIXPublicKey(left)
	rightRaw, rightErr := x509.MarshalPKIXPublicKey(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}

func fiscalAddress(raw []byte) string {
	var address struct {
		Formatted string `json:"formatted"`
	}
	if json.Unmarshal(raw, &address) == nil && strings.TrimSpace(address.Formatted) != "" {
		return address.Formatted
	}
	return string(raw)
}

func apiTaxCondition(raw string) api.ArgentinaTaxCondition {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "responsable_inscripto", "registered":
		return api.ArgentinaTaxConditionRegistered
	case "monotributo":
		return api.ArgentinaTaxConditionMonotributo
	case "no_responsable", "not_responsible":
		return api.ArgentinaTaxConditionNotResponsible
	default:
		return api.ArgentinaTaxConditionExempt
	}
}

func dbTaxCondition(condition api.ArgentinaTaxCondition) string {
	switch condition {
	case api.ArgentinaTaxConditionRegistered:
		return "responsable_inscripto"
	case api.ArgentinaTaxConditionMonotributo:
		return "monotributo"
	case api.ArgentinaTaxConditionNotResponsible:
		return "no_responsable"
	default:
		return "exento"
	}
}
