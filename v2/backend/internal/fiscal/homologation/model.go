// Package homologation runs opt-in, read-only technical interoperability
// checks against ARCA homologation and local fiscal construction checks.
//
// A successful run is technical evidence only. It is not, and must never be
// presented as, an approval or homologation granted by ARCA.
package homologation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const EvidenceNotice = "Evidencia técnica de interoperabilidad; no constituye aprobación ni homologación otorgada por ARCA."

type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type CheckStatus string

const (
	CheckSucceeded CheckStatus = "succeeded"
	CheckFailed    CheckStatus = "failed"
)

type CheckKind string

const (
	CheckConfiguration CheckKind = "configuration"
	CheckCertificate   CheckKind = "certificate"
	CheckWSAA          CheckKind = "wsaa"
	CheckWSFELast      CheckKind = "wsfe_last_authorized"
	CheckLocalMatrix   CheckKind = "local_matrix"
)

type PointOfSale struct {
	Code int
	Name string
}

type Configuration struct {
	LegalName              string
	LegalAddress           string
	ActivityStartDate      time.Time
	ProfileVersion         int64
	CUIT                   string
	IssuerVATCondition     string
	SettingsVersion        int64
	CertificateReference   string
	PrivateKeyReference    string
	CertificateFingerprint string
	CertificateValidFrom   time.Time
	CertificateValidUntil  time.Time
	PointsOfSale           []PointOfSale
}

func ConfigurationFingerprint(configuration Configuration) (string, error) {
	if strings.TrimSpace(configuration.LegalName) == "" ||
		strings.TrimSpace(configuration.LegalAddress) == "" ||
		configuration.ActivityStartDate.IsZero() ||
		configuration.ProfileVersion <= 0 ||
		strings.TrimSpace(configuration.CUIT) == "" ||
		strings.TrimSpace(configuration.IssuerVATCondition) == "" ||
		configuration.SettingsVersion <= 0 ||
		strings.TrimSpace(configuration.CertificateFingerprint) == "" ||
		len(configuration.PointsOfSale) == 0 {
		return "", fmt.Errorf("homologation configuration fingerprint is incomplete")
	}
	points := make([]map[string]any, 0, len(configuration.PointsOfSale))
	for _, point := range configuration.PointsOfSale {
		if point.Code <= 0 || strings.TrimSpace(point.Name) == "" {
			return "", fmt.Errorf("homologation point of sale is incomplete")
		}
		points = append(points, map[string]any{
			"code": point.Code,
			"name": strings.TrimSpace(point.Name),
		})
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version":          1,
		"legal_name":              strings.TrimSpace(configuration.LegalName),
		"legal_address":           strings.TrimSpace(configuration.LegalAddress),
		"activity_start_date":     configuration.ActivityStartDate.UTC().Format("2006-01-02"),
		"profile_version":         configuration.ProfileVersion,
		"cuit":                    strings.TrimSpace(configuration.CUIT),
		"issuer_vat_condition":    strings.TrimSpace(configuration.IssuerVATCondition),
		"settings_version":        configuration.SettingsVersion,
		"certificate_fingerprint": strings.TrimSpace(configuration.CertificateFingerprint),
		"points_of_sale":          points,
	})
	if err != nil {
		return "", fmt.Errorf("encode homologation configuration fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type Check struct {
	Ordinal      int
	Kind         CheckKind
	Name         string
	Status       CheckStatus
	PointOfSale  *int
	VoucherType  *int
	Detail       string
	Evidence     json.RawMessage
	EvidenceHash string
	StartedAt    time.Time
	CompletedAt  time.Time
}

type Completion struct {
	Status                 Status
	CertificateFingerprint string
	ConfigurationHash      string
	PointOfSaleCount       int
	Checks                 []Check
	Evidence               json.RawMessage
	EvidenceHash           string
	CompletedAt            time.Time
}

type Repository interface {
	Start(
		ctx context.Context,
		organizationID uuid.UUID,
		requestedBy string,
		startedAt time.Time,
	) (uuid.UUID, error)
	LoadConfiguration(
		ctx context.Context,
		organizationID uuid.UUID,
		at time.Time,
	) (Configuration, error)
	Complete(
		ctx context.Context,
		organizationID, runID uuid.UUID,
		completion Completion,
	) error
}

type Command struct {
	OrganizationID uuid.UUID
	RequestedBy    string
}

type Result struct {
	RunID                  uuid.UUID `json:"run_id"`
	Status                 Status    `json:"status"`
	CertificateFingerprint string    `json:"certificate_fingerprint,omitempty"`
	PointOfSaleCount       int       `json:"point_of_sale_count"`
	CheckCount             int       `json:"check_count"`
	SuccessCount           int       `json:"success_count"`
	FailureCount           int       `json:"failure_count"`
	EvidenceHash           string    `json:"evidence_sha256"`
	StartedAt              time.Time `json:"started_at"`
	CompletedAt            time.Time `json:"completed_at"`
	Notice                 string    `json:"notice"`
}
