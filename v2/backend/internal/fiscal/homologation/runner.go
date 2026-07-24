package homologation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/authority"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsfev1"
	"github.com/google/uuid"
)

var mvpVoucherTypes = []ar.VoucherType{
	ar.InvoiceA,
	ar.DebitNoteA,
	ar.CreditNoteA,
	ar.InvoiceB,
	ar.DebitNoteB,
	ar.CreditNoteB,
	ar.InvoiceC,
	ar.DebitNoteC,
	ar.CreditNoteC,
}

type Runner struct {
	repository Repository
	kms        fiscal.KMS
	objects    fiscal.ObjectStore
	renderer   fiscal.ArtifactRenderer
	transport  ar.SOAPTransport
	now        func() time.Time
}

func NewRunner(
	repository Repository,
	kms fiscal.KMS,
	objects fiscal.ObjectStore,
	renderer fiscal.ArtifactRenderer,
	transport ar.SOAPTransport,
	now func() time.Time,
) (*Runner, error) {
	if repository == nil || kms == nil || objects == nil || renderer == nil ||
		transport == nil {
		return nil, errors.New("homologation runner dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	return &Runner{
		repository: repository, kms: kms, objects: objects,
		renderer: renderer, transport: transport, now: now,
	}, nil
}

func (runner *Runner) Run(ctx context.Context, command Command) (Result, error) {
	requestedBy := strings.TrimSpace(command.RequestedBy)
	if command.OrganizationID == uuid.Nil || requestedBy == "" {
		return Result{}, errors.New("homologation organization and actor are required")
	}
	startedAt := runner.now().UTC()
	runID, err := runner.repository.Start(
		ctx, command.OrganizationID, requestedBy, startedAt,
	)
	if err != nil {
		return Result{}, err
	}

	var (
		checks                 []Check
		failures               []error
		configuration          Configuration
		certificateFingerprint string
	)
	configurationStarted := runner.now().UTC()
	configuration, err = runner.repository.LoadConfiguration(
		ctx, command.OrganizationID, startedAt,
	)
	if err != nil {
		checks = append(checks, failedCheck(
			CheckConfiguration,
			"active_homologation_configuration",
			nil,
			nil,
			"Configuración de homologación inválida o incompleta.",
			"configuration_unavailable",
			configurationStarted,
			runner.now().UTC(),
		))
		failures = append(failures, fmt.Errorf("load homologation configuration: %w", err))
		return runner.complete(
			ctx, command.OrganizationID, runID, startedAt,
			configuration, certificateFingerprint, checks, failures,
		)
	}
	checks = append(checks, successfulCheck(
		CheckConfiguration,
		"active_homologation_configuration",
		nil,
		nil,
		"Perfil AR, certificado activo y puntos de venta de homologación seleccionados.",
		map[string]any{
			"country_code":        "AR",
			"environment":         string(ar.Homologation),
			"point_of_sale_count": len(configuration.PointsOfSale),
			"issuing_system":      "wsfev1",
		},
		configurationStarted,
		runner.now().UTC(),
	))

	cuit, cuitErr := ar.ParseCUIT(configuration.CUIT)
	certificateStarted := runner.now().UTC()
	certificateObject, objectErr := runner.objects.Get(
		ctx, strings.TrimSpace(configuration.CertificateReference),
	)
	var certificateErr error
	if cuitErr != nil {
		certificateErr = cuitErr
	} else if objectErr != nil {
		certificateErr = objectErr
	} else {
		publicKey, publicKeyErr := runner.kms.PublicKey(
			ctx, strings.TrimSpace(configuration.PrivateKeyReference),
		)
		if publicKeyErr != nil {
			certificateErr = publicKeyErr
		} else {
			info, validationErr := ar.ValidateCertificate(
				certificateObject.Body, publicKey, cuit, startedAt,
			)
			if validationErr != nil {
				certificateErr = validationErr
			} else if info.Fingerprint != strings.TrimSpace(
				configuration.CertificateFingerprint,
			) {
				certificateErr = errors.New(
					"stored certificate fingerprint does not match certificate bytes",
				)
			} else {
				certificateFingerprint = info.Fingerprint
				checks = append(checks, successfulCheck(
					CheckCertificate,
					"certificate_profile_and_key_match",
					nil,
					nil,
					"CUIT, certificado y clave de firma coinciden y están vigentes.",
					map[string]any{
						"fingerprint_sha256":  certificateFingerprint,
						"valid_from":          info.NotBefore.UTC().Format(time.RFC3339),
						"valid_until":         info.NotAfter.UTC().Format(time.RFC3339),
						"cuit_validated":      true,
						"private_key_exposed": false,
					},
					certificateStarted,
					runner.now().UTC(),
				))
			}
		}
	}
	if certificateErr != nil {
		checks = append(checks, failedCheck(
			CheckCertificate,
			"certificate_profile_and_key_match",
			nil,
			nil,
			"El certificado, CUIT o clave de firma no superó la validación.",
			"certificate_validation_failed",
			certificateStarted,
			runner.now().UTC(),
		))
		failures = append(
			failures,
			errors.New("validate homologation certificate: validation failed"),
		)
	}

	var ticket wsaa.AccessTicket
	if certificateErr == nil {
		credentials := wsaa.Credentials{
			OrganizationID: command.OrganizationID,
			Environment:    ar.Homologation,
			Service:        wsaa.ServiceWSFE,
			CUIT:           cuit,
			CertificatePEM: append([]byte(nil), certificateObject.Body...),
			KeyReference:   strings.TrimSpace(configuration.PrivateKeyReference),
		}
		wsaaStarted := runner.now().UTC()
		wsaaClient := wsaa.NewClient(runner.transport)
		wsaaClient.Now = func() time.Time { return startedAt }
		authenticator := &wsaa.Authenticator{
			Client:  wsaaClient,
			Tickets: authority.NewMemoryTickets(),
			KMS:     runner.kms,
			Now:     func() time.Time { return startedAt },
		}
		ticket, err = authenticator.AccessTicket(ctx, credentials)
		if err != nil {
			checks = append(checks, failedCheck(
				CheckWSAA,
				"wsaa_access_ticket",
				nil,
				nil,
				"No fue posible obtener un ticket WSAA de homologación.",
				"wsaa_access_ticket_failed",
				wsaaStarted,
				runner.now().UTC(),
			))
			failures = append(
				failures,
				errors.New("obtain homologation WSAA ticket: remote authentication failed"),
			)
		} else {
			checks = append(checks, successfulCheck(
				CheckWSAA,
				"wsaa_access_ticket",
				nil,
				nil,
				"Ticket WSAA obtenido; token y firma fueron omitidos de la evidencia.",
				map[string]any{
					"environment":             string(ar.Homologation),
					"service":                 wsaa.ServiceWSFE,
					"certificate_fingerprint": certificateFingerprint,
					"expires_at":              ticket.ExpiresAt.UTC().Format(time.RFC3339),
					"credentials_redacted":    true,
				},
				wsaaStarted,
				runner.now().UTC(),
			))
		}
	}

	if ticket.ValidAt(startedAt) {
		wsfeClient, clientErr := wsfev1.NewClient(runner.transport, ar.Homologation)
		if clientErr != nil {
			failures = append(failures, clientErr)
		} else {
			auth := wsfev1.Auth{Ticket: ticket, CUIT: cuit}
			for _, point := range configuration.PointsOfSale {
				for _, voucherType := range mvpVoucherTypes {
					probeStarted := runner.now().UTC()
					lastAuthorized, probeErr := wsfeClient.LastAuthorized(
						ctx, auth, point.Code, voucherType,
					)
					pointCode := point.Code
					typeCode := int(voucherType)
					if probeErr != nil {
						checks = append(checks, failedCheck(
							CheckWSFELast,
							"fe_comp_ultimo_autorizado",
							&pointCode,
							&typeCode,
							"Falló la consulta de sólo lectura de la última numeración.",
							"wsfe_last_authorized_failed",
							probeStarted,
							runner.now().UTC(),
						))
						failures = append(failures, fmt.Errorf(
							"query FECompUltimoAutorizado for POS %d type %d: remote read failed",
							point.Code, voucherType,
						))
						continue
					}
					checks = append(checks, successfulCheck(
						CheckWSFELast,
						"fe_comp_ultimo_autorizado",
						&pointCode,
						&typeCode,
						"Última numeración consultada mediante FECompUltimoAutorizado.",
						map[string]any{
							"environment":            string(ar.Homologation),
							"point_of_sale":          point.Code,
							"voucher_type":           int(voucherType),
							"last_authorized_number": lastAuthorized,
							"arca_operation":         "FECompUltimoAutorizado",
							"read_only":              true,
						},
						probeStarted,
						runner.now().UTC(),
					))
				}
			}
		}
	} else {
		for _, point := range configuration.PointsOfSale {
			for _, voucherType := range mvpVoucherTypes {
				pointCode := point.Code
				typeCode := int(voucherType)
				checks = append(checks, failedCheck(
					CheckWSFELast,
					"fe_comp_ultimo_autorizado",
					&pointCode,
					&typeCode,
					"La consulta no se ejecutó porque WSAA o el certificado falló.",
					"wsfe_probe_dependency_failed",
					runner.now().UTC(),
					runner.now().UTC(),
				))
			}
		}
	}

	// The local matrix is independent from the remote read-only probes. It
	// exercises every A/B/C invoice, credit-note and debit-note type, concepts,
	// association, foreign currency, immutable snapshots, PDF and QR.
	matrixPoint := configuration.PointsOfSale[0].Code
	for _, testCase := range localCases() {
		localStarted := runner.now().UTC()
		evidence, matrixErr := validateLocalMatrixCase(
			ctx, runner.renderer, configuration, matrixPoint, startedAt, testCase,
		)
		typeCode := int(testCase.VoucherType)
		if matrixErr != nil {
			checks = append(checks, failedCheck(
				CheckLocalMatrix,
				testCase.Name,
				&matrixPoint,
				&typeCode,
				"Falló la construcción o validación fiscal local no emisiva.",
				"local_matrix_validation_failed",
				localStarted,
				runner.now().UTC(),
			))
			failures = append(failures, fmt.Errorf(
				"validate local homologation case %s: %w", testCase.Name, matrixErr,
			))
			continue
		}
		checks = append(checks, successfulCheck(
			CheckLocalMatrix,
			testCase.Name,
			&matrixPoint,
			&typeCode,
			"Envelope, snapshot, PDF y QR locales validados sin emitir a ARCA.",
			evidence,
			localStarted,
			runner.now().UTC(),
		))
	}

	return runner.complete(
		ctx, command.OrganizationID, runID, startedAt,
		configuration, certificateFingerprint, checks, failures,
	)
}

func (runner *Runner) complete(
	ctx context.Context,
	organizationID, runID uuid.UUID,
	startedAt time.Time,
	configuration Configuration,
	certificateFingerprint string,
	checks []Check,
	failures []error,
) (Result, error) {
	completedAt := runner.now().UTC()
	configurationHash, configurationErr := ConfigurationFingerprint(configuration)
	if configurationErr != nil && len(failures) == 0 {
		failures = append(failures, configurationErr)
	}
	status := StatusSucceeded
	if len(failures) > 0 {
		status = StatusFailed
	}
	successCount := 0
	failureCount := 0
	evidenceChecks := make([]map[string]any, 0, len(checks))
	for index := range checks {
		checks[index].Ordinal = index + 1
		if checks[index].Status == CheckSucceeded {
			successCount++
		} else {
			failureCount++
		}
		evidenceChecks = append(evidenceChecks, map[string]any{
			"ordinal":         checks[index].Ordinal,
			"kind":            checks[index].Kind,
			"name":            checks[index].Name,
			"status":          checks[index].Status,
			"point_of_sale":   checks[index].PointOfSale,
			"voucher_type":    checks[index].VoucherType,
			"evidence_sha256": checks[index].EvidenceHash,
		})
	}
	evidence, evidenceHash := canonicalEvidence(map[string]any{
		"schema_version":          1,
		"notice":                  EvidenceNotice,
		"environment":             string(ar.Homologation),
		"remote_operations":       []string{"LoginCms", "FECompUltimoAutorizado"},
		"authorization_requested": false,
		"certificate_fingerprint": certificateFingerprint,
		"configuration_sha256":    configurationHash,
		"point_of_sale_count":     len(configuration.PointsOfSale),
		"check_count":             len(checks),
		"success_count":           successCount,
		"failure_count":           failureCount,
		"checks":                  evidenceChecks,
	})
	completion := Completion{
		Status: status, CertificateFingerprint: certificateFingerprint,
		ConfigurationHash: configurationHash,
		PointOfSaleCount:  len(configuration.PointsOfSale), Checks: checks,
		Evidence: evidence, EvidenceHash: evidenceHash, CompletedAt: completedAt,
	}
	persistenceContext, cancelPersistence := context.WithTimeout(
		context.WithoutCancel(ctx), 10*time.Second,
	)
	defer cancelPersistence()
	if err := runner.repository.Complete(
		persistenceContext, organizationID, runID, completion,
	); err != nil {
		failures = append(failures, err)
	}
	result := Result{
		RunID: runID, Status: status,
		CertificateFingerprint: certificateFingerprint,
		PointOfSaleCount:       len(configuration.PointsOfSale),
		CheckCount:             len(checks), SuccessCount: successCount,
		FailureCount: failureCount, EvidenceHash: evidenceHash,
		StartedAt: startedAt, CompletedAt: completedAt, Notice: EvidenceNotice,
	}
	if len(failures) > 0 {
		return result, errors.Join(failures...)
	}
	return result, nil
}

func successfulCheck(
	kind CheckKind,
	name string,
	pointOfSale, voucherType *int,
	detail string,
	evidence any,
	startedAt, completedAt time.Time,
) Check {
	raw, hash := canonicalEvidence(evidence)
	return Check{
		Kind: kind, Name: name, Status: CheckSucceeded,
		PointOfSale: pointOfSale, VoucherType: voucherType,
		Detail: detail, Evidence: raw, EvidenceHash: hash,
		StartedAt: startedAt, CompletedAt: completedAt,
	}
}

func failedCheck(
	kind CheckKind,
	name string,
	pointOfSale, voucherType *int,
	detail, errorCode string,
	startedAt, completedAt time.Time,
) Check {
	raw, hash := canonicalEvidence(map[string]any{
		"error_code": errorCode,
		"redacted":   true,
	})
	return Check{
		Kind: kind, Name: name, Status: CheckFailed,
		PointOfSale: pointOfSale, VoucherType: voucherType,
		Detail: detail, Evidence: raw, EvidenceHash: hash,
		StartedAt: startedAt, CompletedAt: completedAt,
	}
}

func canonicalEvidence(value any) (json.RawMessage, string) {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte(`{"evidence_encoding_failed":true}`)
	}
	sum := sha256.Sum256(raw)
	return json.RawMessage(raw), hex.EncodeToString(sum[:])
}
