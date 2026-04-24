package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	"github.com/google/uuid"
)

const defaultConfirmationTTL = 15 * time.Minute

type RepositoryPort interface {
	CreateConfirmation(ctx context.Context, in Confirmation) (Confirmation, error)
	GetConfirmation(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (Confirmation, error)
	MarkConfirmationUsed(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error
	GetIdempotencyRecord(ctx context.Context, orgID uuid.UUID, actor, capabilityID, key string) (IdempotencyRecord, bool, error)
	SaveIdempotencyRecord(ctx context.Context, in IdempotencyRecord) error
	ListAgentEvents(ctx context.Context, orgID uuid.UUID, limit int, capabilityID, requestID string) ([]AgentEvent, error)
}

type ReviewClient interface {
	SubmitRequest(ctx context.Context, idempotencyKey string, body reviewclient.SubmitRequestBody) (reviewclient.SubmitResponse, error)
	GetRequest(ctx context.Context, id string) (reviewclient.RequestSummary, int, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	registry *Registry
	repo     RepositoryPort
	review   ReviewClient
	audit    AuditPort
}

func NewUsecases(repo RepositoryPort, review ReviewClient, audit AuditPort) *Usecases {
	return &Usecases{registry: NewRegistry(), repo: repo, review: review, audit: audit}
}

func (u *Usecases) ListCapabilities() []Capability {
	return u.registry.List()
}

func (u *Usecases) GetCapability(id string) (Capability, bool) {
	return u.registry.Get(id)
}

type CreateConfirmationInput struct {
	Auth         ActorContext
	CapabilityID string
	Payload      json.RawMessage
	PayloadHash  string
	HumanSummary string
	ExpiresAt    *time.Time
}

type ConfirmationOutput struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	PayloadHash  string    `json:"payload_hash"`
	HumanSummary string    `json:"human_summary"`
	RiskLevel    string    `json:"risk_level"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (u *Usecases) CreateConfirmation(ctx context.Context, in CreateConfirmationInput) (ConfirmationOutput, error) {
	orgID, err := uuid.Parse(strings.TrimSpace(in.Auth.OrgID))
	if err != nil {
		return ConfirmationOutput{}, agentError(http.StatusBadRequest, "invalid_org", "org invalida")
	}
	capability, ok := u.registry.Get(in.CapabilityID)
	if !ok {
		return ConfirmationOutput{}, agentError(http.StatusNotFound, "capability_not_found", "capability no encontrada")
	}
	payloadHash := strings.TrimSpace(in.PayloadHash)
	if payloadHash == "" {
		payloadHash, err = PayloadHashFromRaw(in.Payload)
		if err != nil {
			return ConfirmationOutput{}, agentError(http.StatusBadRequest, "invalid_payload", "payload invalido")
		}
	}
	expiresAt := time.Now().UTC().Add(defaultConfirmationTTL)
	if in.ExpiresAt != nil {
		expiresAt = in.ExpiresAt.UTC()
	}
	if !expiresAt.After(time.Now().UTC()) {
		return ConfirmationOutput{}, agentError(http.StatusBadRequest, "invalid_expiration", "expires_at debe estar en el futuro")
	}
	out, err := u.repo.CreateConfirmation(ctx, Confirmation{
		OrgID:        orgID,
		Actor:        strings.TrimSpace(in.Auth.Actor),
		CapabilityID: capability.ID,
		PayloadHash:  payloadHash,
		HumanSummary: strings.TrimSpace(in.HumanSummary),
		RiskLevel:    string(capability.RiskLevel),
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return ConfirmationOutput{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.Auth.OrgID, in.Auth.Actor, "agent.confirmation.created", "agent_confirmation", out.ID.String(), map[string]any{
			"capability_id": capability.ID,
			"payload_hash":  payloadHash,
			"risk_level":    capability.RiskLevel,
		})
	}
	return confirmationOutput(out), nil
}

type DryRunInput struct {
	Auth         ActorContext
	CapabilityID string
	Payload      json.RawMessage
	Channel      Channel
	Reason       string
}

type DryRunOutput struct {
	Capability              Capability `json:"capability"`
	PayloadHash             string     `json:"payload_hash"`
	Channel                 Channel    `json:"channel"`
	RequiresConfirmation    bool       `json:"requires_confirmation"`
	RequiresReview          bool       `json:"requires_review"`
	RequiresIdempotencyKey  bool       `json:"requires_idempotency_key"`
	NexusGovernanceRequired bool       `json:"nexus_governance_required"`
	ExecutorStatus          string     `json:"executor_status"`
	HumanSummary            string     `json:"human_summary"`
}

func (u *Usecases) DryRun(_ context.Context, in DryRunInput) (DryRunOutput, error) {
	capability, ok := u.registry.Get(in.CapabilityID)
	if !ok {
		return DryRunOutput{}, agentError(http.StatusNotFound, "capability_not_found", "capability no encontrada")
	}
	channel := in.Channel
	if channel == "" {
		channel = defaultChannelForAuth(in.Auth.AuthMethod)
	}
	if !channelAllowed(capability, channel) {
		return DryRunOutput{}, agentError(http.StatusForbidden, "channel_not_allowed", "capability no permitida para este canal")
	}
	payloadHash, err := PayloadHashFromRaw(in.Payload)
	if err != nil {
		return DryRunOutput{}, agentError(http.StatusBadRequest, "invalid_payload", "payload invalido")
	}
	return DryRunOutput{
		Capability:              capability,
		PayloadHash:             payloadHash,
		Channel:                 channel,
		RequiresConfirmation:    capability.RequiresConfirmation,
		RequiresReview:          capability.RequiresReview,
		RequiresIdempotencyKey:  capability.RequiresIdempotencyKey,
		NexusGovernanceRequired: capability.RequiresReview,
		ExecutorStatus:          capability.ExecutorStatus,
		HumanSummary:            buildHumanSummary(capability, payloadHash, in.Reason),
	}, nil
}

type ExecuteInput struct {
	Auth            ActorContext
	CapabilityID    string
	Payload         json.RawMessage
	Channel         Channel
	ConfirmationID  string
	ReviewRequestID string
	Reason          string
	IdempotencyKey  string
	RequestID       string
}

type ExecuteOutput struct {
	Status          string         `json:"status"`
	CapabilityID    string         `json:"capability_id"`
	PayloadHash     string         `json:"payload_hash"`
	IdempotencyKey  string         `json:"idempotency_key,omitempty"`
	ConfirmationID  string         `json:"confirmation_id,omitempty"`
	ReviewRequestID string         `json:"review_request_id,omitempty"`
	ReviewDecision  string         `json:"review_decision,omitempty"`
	ReviewStatus    string         `json:"review_status,omitempty"`
	ExecutorStatus  string         `json:"executor_status"`
	Message         string         `json:"message"`
	Replay          bool           `json:"idempotency_replay,omitempty"`
	Requirements    map[string]any `json:"requirements,omitempty"`
}

type ExecuteResult struct {
	StatusCode int
	Output     ExecuteOutput
}

func (u *Usecases) Execute(ctx context.Context, in ExecuteInput) (ExecuteResult, error) {
	orgID, err := uuid.Parse(strings.TrimSpace(in.Auth.OrgID))
	if err != nil {
		return ExecuteResult{}, agentError(http.StatusBadRequest, "invalid_org", "org invalida")
	}
	capability, ok := u.registry.Get(in.CapabilityID)
	if !ok {
		return ExecuteResult{}, agentError(http.StatusNotFound, "capability_not_found", "capability no encontrada")
	}
	channel := in.Channel
	if channel == "" {
		channel = defaultChannelForAuth(in.Auth.AuthMethod)
	}
	if !channelAllowed(capability, channel) {
		return ExecuteResult{}, agentError(http.StatusForbidden, "channel_not_allowed", "capability no permitida para este canal")
	}
	payloadHash, err := PayloadHashFromRaw(in.Payload)
	if err != nil {
		return ExecuteResult{}, agentError(http.StatusBadRequest, "invalid_payload", "payload invalido")
	}
	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey == "" && strings.EqualFold(in.Auth.AuthMethod, "api_key") {
		idempotencyKey = strings.TrimSpace(in.RequestID)
	}
	if capability.RequiresIdempotencyKey && idempotencyKey == "" {
		return ExecuteResult{}, agentError(http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key requerido")
	}
	if idempotencyKey != "" {
		existing, found, err := u.repo.GetIdempotencyRecord(ctx, orgID, strings.TrimSpace(in.Auth.Actor), capability.ID, idempotencyKey)
		if err != nil {
			return ExecuteResult{}, err
		}
		if found {
			if existing.PayloadHash != payloadHash {
				return ExecuteResult{}, agentError(http.StatusConflict, "idempotency_key_payload_mismatch", "misma idempotency key con payload distinto")
			}
			var replay ExecuteOutput
			if err := json.Unmarshal(existing.Response, &replay); err != nil {
				return ExecuteResult{}, err
			}
			replay.Replay = true
			return ExecuteResult{StatusCode: existing.StatusCode, Output: replay}, nil
		}
	}

	if capability.RequiresConfirmation {
		if err := u.validateConfirmation(ctx, orgID, strings.TrimSpace(in.Auth.Actor), capability, payloadHash, in.ConfirmationID); err != nil {
			return ExecuteResult{}, err
		}
	}

	reviewRequestID := strings.TrimSpace(in.ReviewRequestID)
	reviewDecision := ""
	reviewStatus := ""
	if capability.RequiresReview {
		if u.review == nil {
			return ExecuteResult{}, agentError(http.StatusServiceUnavailable, "review_unavailable", "Nexus Review no esta configurado")
		}
		if reviewRequestID == "" {
			resp, err := u.review.SubmitRequest(ctx, idempotencyKey, reviewclient.SubmitRequestBody{
				RequesterType:  requesterType(in.Auth.AuthMethod),
				RequesterID:    strings.TrimSpace(in.Auth.Actor),
				RequesterName:  strings.TrimSpace(in.Auth.Actor),
				ActionType:     capability.NexusActionType,
				TargetSystem:   "pymes",
				TargetResource: capability.Resource,
				Reason:         defaultString(strings.TrimSpace(in.Reason), "agent capability execution"),
				Context:        "pymes-core agent capability gateway",
				Params: map[string]any{
					"capability_id":     capability.ID,
					"payload_hash":      payloadHash,
					"payload":           decodePayload(in.Payload),
					"confirmation_id":   strings.TrimSpace(in.ConfirmationID),
					"idempotency_key":   idempotencyKey,
					"channel":           channel,
					"actor":             strings.TrimSpace(in.Auth.Actor),
					"request_id":        strings.TrimSpace(in.RequestID),
					"executor_status":   capability.ExecutorStatus,
					"requires_callback": true,
				},
			})
			if err != nil {
				return ExecuteResult{}, err
			}
			reviewRequestID = resp.RequestID
			reviewDecision = resp.Decision
			reviewStatus = resp.Status
			if !reviewAllows(resp.Decision, resp.Status) {
				output := ExecuteOutput{
					Status:          "pending_review",
					CapabilityID:    capability.ID,
					PayloadHash:     payloadHash,
					IdempotencyKey:  idempotencyKey,
					ConfirmationID:  strings.TrimSpace(in.ConfirmationID),
					ReviewRequestID: reviewRequestID,
					ReviewDecision:  reviewDecision,
					ReviewStatus:    reviewStatus,
					ExecutorStatus:  capability.ExecutorStatus,
					Message:         "Nexus Review debe aprobar esta accion antes de ejecutarla.",
					Requirements: map[string]any{
						"review":       true,
						"confirmation": capability.RequiresConfirmation,
					},
				}
				if err := u.saveIdempotency(ctx, orgID, in.Auth.Actor, capability.ID, idempotencyKey, payloadHash, http.StatusAccepted, output); err != nil {
					return ExecuteResult{}, err
				}
				u.logExecution(ctx, in.Auth, capability, payloadHash, "agent.action.pending_review", reviewRequestID, idempotencyKey)
				return ExecuteResult{StatusCode: http.StatusAccepted, Output: output}, nil
			}
		}
	}

	output := ExecuteOutput{
		Status:          "executor_not_registered",
		CapabilityID:    capability.ID,
		PayloadHash:     payloadHash,
		IdempotencyKey:  idempotencyKey,
		ConfirmationID:  strings.TrimSpace(in.ConfirmationID),
		ReviewRequestID: reviewRequestID,
		ReviewDecision:  reviewDecision,
		ReviewStatus:    reviewStatus,
		ExecutorStatus:  capability.ExecutorStatus,
		Message:         "La capability esta registrada y gobernada, pero todavia no tiene executor de dominio conectado.",
	}
	if err := u.saveIdempotency(ctx, orgID, in.Auth.Actor, capability.ID, idempotencyKey, payloadHash, http.StatusNotImplemented, output); err != nil {
		return ExecuteResult{}, err
	}
	u.logExecution(ctx, in.Auth, capability, payloadHash, "agent.action.executor_missing", reviewRequestID, idempotencyKey)
	return ExecuteResult{StatusCode: http.StatusNotImplemented, Output: output}, nil
}

func (u *Usecases) ListEvents(ctx context.Context, auth ActorContext, limit int, capabilityID, requestID string) ([]AgentEvent, error) {
	orgID, err := uuid.Parse(strings.TrimSpace(auth.OrgID))
	if err != nil {
		return nil, agentError(http.StatusBadRequest, "invalid_org", "org invalida")
	}
	return u.repo.ListAgentEvents(ctx, orgID, limit, strings.TrimSpace(capabilityID), strings.TrimSpace(requestID))
}

func (u *Usecases) validateConfirmation(ctx context.Context, orgID uuid.UUID, actor string, capability Capability, payloadHash, rawID string) error {
	if strings.TrimSpace(rawID) == "" {
		return agentError(http.StatusPreconditionRequired, "confirmation_required", "confirmation_id requerido")
	}
	id, err := uuid.Parse(strings.TrimSpace(rawID))
	if err != nil {
		return agentError(http.StatusBadRequest, "invalid_confirmation", "confirmation_id invalido")
	}
	conf, err := u.repo.GetConfirmation(ctx, orgID, id)
	if err != nil {
		return agentError(http.StatusNotFound, "confirmation_not_found", "confirmacion no encontrada")
	}
	if conf.Actor != actor {
		return agentError(http.StatusForbidden, "confirmation_actor_mismatch", "confirmacion creada por otro actor")
	}
	if conf.CapabilityID != capability.ID {
		return agentError(http.StatusConflict, "confirmation_capability_mismatch", "confirmacion no corresponde a la capability")
	}
	if conf.Status != "pending" || time.Now().UTC().After(conf.ExpiresAt) {
		return agentError(http.StatusConflict, "confirmation_expired_or_used", "confirmacion expirada o usada")
	}
	if conf.PayloadHash != payloadHash {
		return agentError(http.StatusConflict, "confirmation_payload_mismatch", "payload no coincide con la confirmacion")
	}
	return nil
}

func (u *Usecases) saveIdempotency(ctx context.Context, orgID uuid.UUID, actor, capabilityID, key, payloadHash string, status int, output ExecuteOutput) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return err
	}
	return u.repo.SaveIdempotencyRecord(ctx, IdempotencyRecord{
		OrgID:          orgID,
		Actor:          strings.TrimSpace(actor),
		CapabilityID:   capabilityID,
		IdempotencyKey: strings.TrimSpace(key),
		PayloadHash:    payloadHash,
		Response:       raw,
		StatusCode:     status,
	})
}

func (u *Usecases) logExecution(ctx context.Context, auth ActorContext, capability Capability, payloadHash, action, reviewRequestID, idempotencyKey string) {
	if u.audit == nil {
		return
	}
	u.audit.Log(ctx, auth.OrgID, auth.Actor, action, "agent_capability", capability.ID, map[string]any{
		"capability_id":     capability.ID,
		"payload_hash":      payloadHash,
		"review_request_id": reviewRequestID,
		"idempotency_key":   idempotencyKey,
		"risk_level":        capability.RiskLevel,
	})
}

func confirmationOutput(conf Confirmation) ConfirmationOutput {
	return ConfirmationOutput{
		ID:           conf.ID.String(),
		CapabilityID: conf.CapabilityID,
		PayloadHash:  conf.PayloadHash,
		HumanSummary: conf.HumanSummary,
		RiskLevel:    conf.RiskLevel,
		Status:       conf.Status,
		ExpiresAt:    conf.ExpiresAt,
		CreatedAt:    conf.CreatedAt,
	}
}

func buildHumanSummary(capability Capability, payloadHash, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "sin motivo declarado"
	}
	return fmt.Sprintf("%s requiere %s sobre %s. payload_hash=%s. motivo=%s", capability.ID, capability.Action, capability.Resource, payloadHash, reason)
}

func defaultChannelForAuth(authMethod string) Channel {
	if strings.EqualFold(strings.TrimSpace(authMethod), "api_key") {
		return ChannelExternalAgent
	}
	return ChannelHumanUI
}

func requesterType(authMethod string) string {
	if strings.EqualFold(strings.TrimSpace(authMethod), "api_key") {
		return "agent"
	}
	return "human"
}

func reviewAllows(decision, status string) bool {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "allow", "allowed", "approve", "approved":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "allowed", "approved":
		return true
	default:
		return false
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

type codedError struct {
	status  int
	code    string
	message string
}

func (e codedError) Error() string { return e.message }

func agentError(status int, code, message string) error {
	return codedError{status: status, code: code, message: message}
}

func errorStatus(err error) (int, string, string, bool) {
	var coded codedError
	if errors.As(err, &coded) {
		return coded.status, coded.code, coded.message, true
	}
	return 0, "", "", false
}
