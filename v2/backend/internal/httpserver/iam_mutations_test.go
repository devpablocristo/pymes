package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	mutationActorMembership = "11111111-1111-4111-8111-111111111111"
	mutationActorUser       = "22222222-2222-4222-8222-222222222222"
	mutationOrganization    = "33333333-3333-4333-8333-333333333333"
	mutationTargetMember    = "44444444-4444-4444-8444-444444444444"
	mutationTargetUser      = "55555555-5555-4555-8555-555555555555"
)

type mutationOutbox struct {
	input platformoutbox.MessageInput
	tx    pgx.Tx
	calls int
	err   error
}

func (outbox *mutationOutbox) Append(
	_ context.Context,
	tx pgx.Tx,
	input platformoutbox.MessageInput,
) (platformoutbox.Message, error) {
	outbox.calls++
	outbox.tx = tx
	outbox.input = input
	if outbox.err != nil {
		return platformoutbox.Message{}, outbox.err
	}
	return platformoutbox.Message{
		ID:             "outbox-message",
		IdempotencyKey: input.IdempotencyKey,
		Topic:          input.Topic,
		Payload:        input.Payload,
	}, nil
}

type mutationTx struct {
	pgx.Tx
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	queryRow func(context.Context, string, ...any) pgx.Row
}

func (tx *mutationTx) Exec(
	ctx context.Context,
	query string,
	args ...any,
) (pgconn.CommandTag, error) {
	if tx.exec == nil {
		panic("unexpected Exec call")
	}
	return tx.exec(ctx, query, args...)
}

func (tx *mutationTx) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) pgx.Row {
	if tx.queryRow == nil {
		panic("unexpected QueryRow call")
	}
	return tx.queryRow(ctx, query, args...)
}

type mutationRow struct {
	values []any
	err    error
}

func (row mutationRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return errors.New("unexpected mutation row destination count")
	}
	for index, value := range row.values {
		switch destination := destinations[index].(type) {
		case *string:
			*destination = value.(string)
		case *[]byte:
			*destination = bytes.Clone(value.([]byte))
		case *bool:
			*destination = value.(bool)
		case *time.Time:
			*destination = value.(time.Time)
		default:
			return errors.New("unsupported mutation scan destination")
		}
	}
	return nil
}

type mutationTransactor struct {
	tx         pgx.Tx
	active     platformiam.ActiveMembership
	committed  bool
	rolledBack bool
}

func (transactor *mutationTransactor) WithinSessionTx(
	ctx context.Context,
	_ platformiam.VerifiedSession,
	callback platformiam.SessionTxFunc,
) error {
	err := callback(ctx, transactor.tx, transactor.active)
	if err != nil {
		transactor.rolledBack = true
		return err
	}
	transactor.committed = true
	return nil
}

func TestUpdateTeamMemberAppliesReductionsAndQueuesElevations(t *testing.T) {
	tests := []struct {
		name           string
		currentRole    string
		desiredRole    string
		wantRole       api.Role
		wantLocalWrite bool
		wantApplied    bool
	}{
		{
			name:           "admin to member is fail-closed locally first",
			currentRole:    "admin",
			desiredRole:    "member",
			wantRole:       api.RoleMember,
			wantLocalWrite: true,
			wantApplied:    true,
		},
		{
			name:           "member to admin waits for provider elevation",
			currentRole:    "member",
			desiredRole:    "admin",
			wantRole:       api.RoleMember,
			wantLocalWrite: false,
			wantApplied:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queryCalls := 0
			roleWrites := 0
			tx := &mutationTx{}
			tx.exec = func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
				if strings.Contains(query, "pg_advisory_xact_lock") {
					if len(args) != 1 {
						t.Fatalf("advisory lock args = %v", args)
					}
					return pgconn.NewCommandTag("SELECT 1"), nil
				}
				if strings.Contains(query, "UPDATE iam.memberships") {
					roleWrites++
					if strings.Contains(strings.ToLower(query), "org_id") {
						t.Fatalf("member mutation bypasses RLS: %s", query)
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				}
				t.Fatalf("unexpected Exec query: %s", query)
				return pgconn.CommandTag{}, nil
			}
			tx.queryRow = func(_ context.Context, query string, args ...any) pgx.Row {
				queryCalls++
				switch queryCalls {
				case 1:
					if !strings.Contains(query, "platform_outbox_messages") {
						t.Fatalf("first query is not replay lookup: %s", query)
					}
					return mutationRow{err: pgx.ErrNoRows}
				case 2:
					if !strings.Contains(query, "FROM iam.memberships") ||
						strings.Contains(strings.ToLower(query), "org_id") {
						t.Fatalf("target lookup does not rely exclusively on RLS: %s", query)
					}
					return mutationMemberRow(test.currentRole, "active", "clerk_membership")
				default:
					t.Fatalf("unexpected QueryRow call %d: %s %v", queryCalls, query, args)
					return mutationRow{err: pgx.ErrNoRows}
				}
			}
			outbox := &mutationOutbox{}
			transactor := &mutationTransactor{
				tx:     tx,
				active: mutationActiveMembership("owner"),
			}
			handler := newMutationHandler(t, transactor, outbox)
			response := performMutationRequest(
				handler,
				http.MethodPatch,
				"/api/v1/team/members/"+mutationTargetMember,
				"member-role-0001",
				`{"role":"`+test.desiredRole+`"}`,
			)

			if response.Code != http.StatusAccepted {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
			member := decodeIAMIdentityResponse[api.Member](t, response)
			if member.Role != test.wantRole ||
				member.Status != api.MembershipStatusActive ||
				member.SyncStatus != api.SyncStatusQueued {
				t.Fatalf("member response = %+v", member)
			}
			if roleWrites != boolInt(test.wantLocalWrite) {
				t.Fatalf("local role writes = %d, want write=%t", roleWrites, test.wantLocalWrite)
			}
			if !transactor.committed || transactor.rolledBack ||
				outbox.calls != 1 || outbox.tx != tx {
				t.Fatalf(
					"commit=%t rollback=%t outbox calls=%d same tx=%t",
					transactor.committed,
					transactor.rolledBack,
					outbox.calls,
					outbox.tx == tx,
				)
			}
			if outbox.input.Topic != memberRoleChangeTopic ||
				outbox.input.IdempotencyKey != iamCommandKey(
					mutationOrganization,
					mutationActorUser,
					memberRoleChangeOperation,
					"member-role-0001",
				) {
				t.Fatalf("outbox input = %+v", outbox.input)
			}
			event := decodeMutationEvent(t, outbox.input.Payload)
			if event.ResourceID != mutationTargetMember ||
				event.Role != test.desiredRole ||
				event.PreviousRole != test.currentRole ||
				event.AppliedLocally != test.wantApplied {
				t.Fatalf("queued event = %+v", event)
			}
		})
	}
}

func TestUpdateTeamMemberOutboxFailureRollsBackLocalReduction(t *testing.T) {
	queryCalls := 0
	tx := &mutationTx{
		exec: func(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
			switch {
			case strings.Contains(query, "pg_advisory_xact_lock"):
				return pgconn.NewCommandTag("SELECT 1"), nil
			case strings.Contains(query, "UPDATE iam.memberships"):
				return pgconn.NewCommandTag("UPDATE 1"), nil
			default:
				t.Fatalf("unexpected Exec query: %s", query)
				return pgconn.CommandTag{}, nil
			}
		},
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			queryCalls++
			if queryCalls == 1 {
				return mutationRow{err: pgx.ErrNoRows}
			}
			return mutationMemberRow("admin", "active", "clerk_membership")
		},
	}
	transactor := &mutationTransactor{
		tx:     tx,
		active: mutationActiveMembership("owner"),
	}
	outbox := &mutationOutbox{err: errors.New("synthetic outbox failure")}
	response := performMutationRequest(
		newMutationHandler(t, transactor, outbox),
		http.MethodPatch,
		"/api/v1/team/members/"+mutationTargetMember,
		"rollback-role-01",
		`{"role":"member"}`,
	)

	assertAPIError(t, response, http.StatusServiceUnavailable, "IAM_UNAVAILABLE")
	if !transactor.rolledBack || transactor.committed || outbox.calls != 1 {
		t.Fatalf(
			"commit=%t rollback=%t outbox calls=%d",
			transactor.committed,
			transactor.rolledBack,
			outbox.calls,
		)
	}
}

func TestUpdateTeamMemberReplaysOutboxCommandWithoutSecondAppend(t *testing.T) {
	event := iamCommandEvent{
		SchemaVersion:          1,
		Operation:              memberRoleChangeOperation,
		OrganizationID:         mutationOrganization,
		ExternalOrganizationID: "org_clerk",
		ActorUserID:            mutationActorUser,
		ActorMembershipID:      mutationActorMembership,
		ResourceID:             mutationTargetMember,
		Role:                   "admin",
		PreviousRole:           "member",
		AppliedLocally:         false,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	queryCalls := 0
	tx := &mutationTx{
		exec: func(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(query, "pg_advisory_xact_lock") {
				t.Fatalf("unexpected replay Exec: %s", query)
			}
			return pgconn.NewCommandTag("SELECT 1"), nil
		},
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			queryCalls++
			if queryCalls == 1 {
				return mutationRow{
					values: []any{memberRoleChangeTopic, payload},
				}
			}
			return mutationMemberRow("member", "active", "clerk_membership")
		},
	}
	outbox := &mutationOutbox{}
	response := performMutationRequest(
		newMutationHandler(t, &mutationTransactor{
			tx:     tx,
			active: mutationActiveMembership("owner"),
		}, outbox),
		http.MethodPatch,
		"/api/v1/team/members/"+mutationTargetMember,
		"replay-role-0001",
		`{"role":"admin"}`,
	)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if outbox.calls != 0 || queryCalls != 2 {
		t.Fatalf("outbox calls = %d, query calls = %d", outbox.calls, queryCalls)
	}
}

func TestTransferOwnershipIsRejectedBecauseOwnerIsProductWide(t *testing.T) {
	response := performMutationRequest(
		newMutationHandler(t, &mutationTransactor{
			tx:     &mutationTx{},
			active: mutationActiveMembership("owner"),
		}, &mutationOutbox{}),
		http.MethodPost,
		"/api/v1/team/ownership-transfer",
		"ownership-safe-0001",
		`{"member_id":"`+mutationTargetMember+`"}`,
	)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if !strings.Contains(response.Body.String(), "product-wide") {
		t.Fatalf("body = %s", response.Body)
	}
}

func TestCreateInvitationEnforcesActorMatrixBeforePersistence(t *testing.T) {
	tx := &mutationTx{
		exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			t.Fatal("forbidden invitation reached persistence")
			return pgconn.CommandTag{}, nil
		},
		queryRow: func(context.Context, string, ...any) pgx.Row {
			t.Fatal("forbidden invitation reached persistence")
			return mutationRow{err: pgx.ErrNoRows}
		},
	}
	outbox := &mutationOutbox{}
	response := performMutationRequest(
		newMutationHandler(t, &mutationTransactor{
			tx:     tx,
			active: mutationActiveMembership("admin"),
		}, outbox),
		http.MethodPost,
		"/api/v1/team/invitations",
		"invite-admin-001",
		`{"email":"admin@example.com","role":"admin"}`,
	)

	assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
	if outbox.calls != 0 {
		t.Fatalf("outbox calls = %d", outbox.calls)
	}
}

func TestAdminCannotManageAnAdminInvitation(t *testing.T) {
	queryCalls := 0
	tx := &mutationTx{
		exec: func(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(query, "pg_advisory_xact_lock") {
				t.Fatalf("unexpected admin invitation mutation: %s", query)
			}
			return pgconn.NewCommandTag("SELECT 1"), nil
		},
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			queryCalls++
			switch queryCalls {
			case 1:
				return mutationRow{err: pgx.ErrNoRows}
			case 2:
				return mutationRow{values: []any{
					mutationTargetMember,
					"admin-invite@example.test",
					"admin",
					"pending",
					"clerk_admin_invitation",
					time.Date(2026, time.July, 30, 15, 0, 0, 0, time.UTC),
				}}
			default:
				t.Fatalf("unexpected invitation query %d", queryCalls)
				return mutationRow{err: pgx.ErrNoRows}
			}
		},
	}
	outbox := &mutationOutbox{}
	response := performMutationRequest(
		newMutationHandler(t, &mutationTransactor{
			tx:     tx,
			active: mutationActiveMembership("admin"),
		}, outbox),
		http.MethodPost,
		"/api/v1/team/invitations/"+mutationTargetMember+"/revoke",
		"admin-revoke-0001",
		"",
	)

	assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
	if outbox.calls != 0 {
		t.Fatalf("admin queued %d changes for an admin invitation", outbox.calls)
	}
}

func TestIAMCommandValidationRejectsInvalidBodiesAndKeys(t *testing.T) {
	t.Run("unknown JSON field", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodPatch,
			"/api/v1/organization",
			strings.NewReader(`{"name":"Norte","org_id":"forbidden"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		var input api.UpdateOrganizationInput
		if decodeIAMCommandBody(response, request, &input) {
			t.Fatal("unknown field was accepted")
		}
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("multiple JSON values", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodPatch,
			"/api/v1/organization",
			strings.NewReader(`{"name":"Norte"} {"name":"Sur"}`),
		)
		var input api.UpdateOrganizationInput
		if decodeIAMCommandBody(response, request, &input) {
			t.Fatal("multiple JSON values were accepted")
		}
	})

	t.Run("oversized JSON", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodPatch,
			"/api/v1/organization",
			strings.NewReader(`{"name":"`+strings.Repeat("x", maxIAMCommandBody)+`"}`),
		)
		var input api.UpdateOrganizationInput
		if decodeIAMCommandBody(response, request, &input) {
			t.Fatal("oversized JSON was accepted")
		}
	})

	for _, key := range []string{"short", " leading-key", "non-ascii-á"} {
		t.Run("invalid key "+key, func(t *testing.T) {
			response := httptest.NewRecorder()
			if _, ok := validateIdempotencyKey(response, key); ok {
				t.Fatalf("key %q was accepted", key)
			}
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestNormalizeInvitationEmail(t *testing.T) {
	got, err := normalizeInvitationEmail("  Owner@Example.COM ")
	if err != nil || got != "owner@example.com" {
		t.Fatalf("normalized email = %q, %v", got, err)
	}
	for _, invalid := range []string{
		"",
		"Owner <owner@example.com>",
		"not-an-email",
		strings.Repeat("a", 321),
	} {
		if _, err := normalizeInvitationEmail(invalid); err == nil {
			t.Errorf("normalizeInvitationEmail(%q) accepted invalid value", invalid)
		}
	}
}

func TestPendingInvitationConflictClassification(t *testing.T) {
	if !isPendingInvitationConflict(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "iam_invitations_pending_email_uidx",
	}) {
		t.Fatal("pending invitation unique violation was not classified")
	}
	if isPendingInvitationConflict(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "another_unique_index",
	}) {
		t.Fatal("unrelated unique violation was classified as a pending invitation")
	}
}

func newMutationHandler(
	t *testing.T,
	transactor SessionTransactor,
	outbox OutboxAppender,
) http.Handler {
	t.Helper()
	claims := mutationClaims()
	verifier := clerkSessionVerifierFunc(func(
		_ context.Context,
		token string,
	) (clerkadapter.SessionClaims, error) {
		if token != "mutation-token" {
			t.Fatalf("token = %q", token)
		}
		return claims, nil
	})
	return NewHandlerWithIAM(
		discardLogger(),
		nil,
		time.Second,
		NewIAMAPI(mutationClerkConfig(), IAMDependencies{
			Verifier:       verifier,
			Transactor:     transactor,
			OutboxAppender: outbox,
			Now: func() time.Time {
				return time.Date(2026, time.July, 23, 15, 0, 0, 0, time.UTC)
			},
		}),
	)
}

func mutationClerkConfig() config.ClerkConfig {
	return config.ClerkConfig{
		PublishableKey: "pk_test_mutation",
		SecretKey:      "sk_test_mutation",
		Issuer:         "https://clerk.mutation.test",
	}
}

func mutationClaims() clerkadapter.SessionClaims {
	now := time.Date(2026, time.July, 23, 15, 0, 0, 0, time.UTC)
	return clerkadapter.SessionClaims{
		Subject:          "clerk_user_mutation",
		SessionID:        "clerk_session_mutation",
		OrganizationID:   "clerk_org_mutation",
		OrganizationRole: "org:admin",
		IssuedAt:         now.Add(-time.Minute),
		ExpiresAt:        now.Add(time.Minute),
	}
}

func mutationActiveMembership(role string) platformiam.ActiveMembership {
	return platformiam.ActiveMembership{
		MembershipID:   mutationActorMembership,
		OrganizationID: mutationOrganization,
		UserID:         mutationActorUser,
		Role:           role,
	}
}

func mutationMemberRow(role, status, externalID string) pgx.Row {
	return mutationRow{values: []any{
		mutationTargetMember,
		role,
		status,
		externalID,
		mutationTargetUser,
		"target@example.com",
		"Target User",
		"",
	}}
}

func performMutationRequest(
	handler http.Handler,
	method string,
	target string,
	idempotencyKey string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer mutation-token")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeMutationEvent(t *testing.T, payload []byte) iamCommandEvent {
	t.Helper()
	var event iamCommandEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode queued event: %v", err)
	}
	return event
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
