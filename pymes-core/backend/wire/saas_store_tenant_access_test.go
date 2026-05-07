package wire

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
)

type fakeClerkTenantClient struct {
	membershipOK bool
}

func (f fakeClerkTenantClient) CreateOrganization(_ context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error) {
	return clerkOrganization{ID: "org_" + input.Slug}, nil
}

func (f fakeClerkTenantClient) CreateOrganizationInvitation(_ context.Context, _ clerkCreateOrganizationInvitationInput) (clerkOrganizationInvitation, error) {
	return clerkOrganizationInvitation{ID: "clerk_invite_test"}, nil
}

func (f fakeClerkTenantClient) RevokeOrganizationInvitation(_ context.Context, _ clerkRevokeOrganizationInvitationInput) error {
	return nil
}

func (f fakeClerkTenantClient) UserHasOrganizationMembership(_ context.Context, _, _ string) (bool, error) {
	return f.membershipOK, nil
}

func TestFindActiveMembershipRoleByExternalUserUsesLocalRole(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	tenantID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	user, err := store.upsertUserTx(ctx, db, "user_member", "member@bicimax.test", "Member", nil)
	if err != nil {
		t.Fatalf("upsertUserTx() error = %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, tenantID, user.ID.String(), "member"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}

	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, tenantID, "user_member")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "member" {
		t.Fatalf("role = %q ok=%v, want member true", role, ok)
	}
}

func TestFindTenantBySlugForExternalUserFindsExistingOwnedTenant(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	tenantID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}

	row, role, ok, err := store.FindTenantBySlugForExternalUser(ctx, "medlab", "user_owner")
	if err != nil {
		t.Fatalf("FindTenantBySlugForExternalUser() error = %v", err)
	}
	if !ok {
		t.Fatal("FindTenantBySlugForExternalUser() ok = false, want true")
	}
	if row.ID.String() != tenantID {
		t.Fatalf("tenant id = %q, want %q", row.ID.String(), tenantID)
	}
	if role != "owner" {
		t.Fatalf("role = %q, want owner", role)
	}
	if row.ClerkOrgID != nil {
		t.Fatalf("ClerkOrgID = %q, want nil for local fallback tenant", *row.ClerkOrgID)
	}
}

func TestTransferTenantOwnershipKeepsExactlyOneOwner(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	tenantID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	next, err := store.upsertUserTx(ctx, db, "user_next_owner", "next@bicimax.test", "Next Owner", nil)
	if err != nil {
		t.Fatalf("upsertUserTx() error = %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, tenantID, next.ID.String(), "admin"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}

	if err := store.TransferTenantOwnership(ctx, tenantID, "user_owner", next.ID.String()); err != nil {
		t.Fatalf("TransferTenantOwnership() error = %v", err)
	}

	tenantUUID := uuid.MustParse(tenantID)
	var owners int64
	if err := db.Model(&pymesTenantMembershipRow{}).
		Where("tenant_id = ? AND role = 'owner' AND status = 'active'", tenantUUID).
		Count(&owners).Error; err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if owners != 1 {
		t.Fatalf("active owners = %d, want 1", owners)
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, tenantID, "user_next_owner")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "owner" {
		t.Fatalf("next owner role = %q ok=%v, want owner true", role, ok)
	}
}

func TestAcceptTenantInvitationCreatesMembershipForCorrectTenant(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	tenantID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	owner, ok, err := store.LocalUserByExternalID(ctx, "user_owner")
	if err != nil {
		t.Fatalf("LocalUserByExternalID() error = %v", err)
	}
	if !ok {
		t.Fatal("LocalUserByExternalID() ok = false, want true")
	}
	token := "invite-token-test"
	inviteID := uuid.New()
	now := time.Now().UTC()
	row := pymesTenantInvitationRow{
		ID:                inviteID,
		TenantID:          uuid.MustParse(tenantID),
		EmailNormalized:   "new@bicimax.test",
		Role:              "member",
		Status:            "pending",
		TokenHash:         hashInviteToken(token),
		ClerkInvitationID: stringPtr("clerk_invite_test"),
		InvitedByUserID:   owner.ID,
		ExpiresAt:         now.Add(time.Hour),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create invite: %v", err)
	}

	accepted, clerkTenantID, err := store.AcceptTenantInvitation(ctx, token, clerkAuthenticatedUser{
		ExternalID: "user_new",
		Email:      "new@bicimax.test",
		Name:       "New Member",
	})
	if err != nil {
		t.Fatalf("AcceptTenantInvitation() error = %v", err)
	}
	if accepted.Status != "accepted" {
		t.Fatalf("accepted status = %q, want accepted", accepted.Status)
	}
	if clerkTenantID != "org_bicimax" {
		t.Fatalf("clerkTenantID = %q, want org_bicimax", clerkTenantID)
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, tenantID, "user_new")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "member" {
		t.Fatalf("accepted role = %q ok=%v, want member true", role, ok)
	}
}

func TestAcceptTenantInvitationRejectsEmailMismatch(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	tenantID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	owner, ok, err := store.LocalUserByExternalID(ctx, "user_owner")
	if err != nil {
		t.Fatalf("LocalUserByExternalID() error = %v", err)
	}
	if !ok {
		t.Fatal("LocalUserByExternalID() ok = false, want true")
	}
	token := "invite-token-mismatch"
	now := time.Now().UTC()
	if err := db.Create(&pymesTenantInvitationRow{
		ID:              uuid.New(),
		TenantID:        uuid.MustParse(tenantID),
		EmailNormalized: "expected@bicimax.test",
		Role:            "admin",
		Status:          "pending",
		TokenHash:       hashInviteToken(token),
		InvitedByUserID: owner.ID,
		ExpiresAt:       now.Add(time.Hour),
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, _, err = store.AcceptTenantInvitation(ctx, token, clerkAuthenticatedUser{
		ExternalID: "user_wrong",
		Email:      "wrong@bicimax.test",
		Name:       "Wrong User",
	})
	if !errors.Is(err, domainerr.Forbidden("")) {
		t.Fatalf("AcceptTenantInvitation() error = %v, want forbidden", err)
	}
}
