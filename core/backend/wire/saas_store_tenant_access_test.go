package wire

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
)

type fakeClerkTenantClient struct {
	membershipOK                   bool
	createOrgErr                   error
	createInviteErr                error
	getUserErr                     error
	userProfile                    clerkUserProfile
	lastCreateOrg                  clerkCreateOrganizationInput
	lastInviteInput                clerkCreateOrganizationInvitationInput
	lastDeletedOrgMembershipID     string
	lastDeletedOrgMembershipUserID string
	createOrgN                     int
	createInviteN                  int
	getUserN                       int
	deleteMembershipN              int
}

func (f *fakeClerkTenantClient) CreateOrganization(_ context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error) {
	f.lastCreateOrg = input
	f.createOrgN++
	if f.createOrgErr != nil {
		return clerkOrganization{}, f.createOrgErr
	}
	id := "org_" + strings.ReplaceAll(strings.ToLower(strings.TrimSpace(input.Name)), " ", "_")
	if id == "org_" {
		id = "org_test"
	}
	return clerkOrganization{ID: id, Name: strings.TrimSpace(input.Name)}, nil
}

func (f *fakeClerkTenantClient) CreateOrganizationInvitation(_ context.Context, input clerkCreateOrganizationInvitationInput) (clerkOrganizationInvitation, error) {
	f.lastInviteInput = input
	f.createInviteN++
	if f.createInviteErr != nil {
		return clerkOrganizationInvitation{}, f.createInviteErr
	}
	return clerkOrganizationInvitation{ID: "clerk_invite_test"}, nil
}

func (f *fakeClerkTenantClient) GetUser(_ context.Context, userID string) (clerkUserProfile, error) {
	f.getUserN++
	if f.getUserErr != nil {
		return clerkUserProfile{}, f.getUserErr
	}
	if strings.TrimSpace(f.userProfile.ID) == "" {
		f.userProfile.ID = userID
	}
	return f.userProfile, nil
}

func (f *fakeClerkTenantClient) DeleteOrganization(_ context.Context, _ string) error {
	return nil
}

func (f *fakeClerkTenantClient) DeleteOrganizationMembership(_ context.Context, organizationID, userID string) error {
	f.lastDeletedOrgMembershipID = organizationID
	f.lastDeletedOrgMembershipUserID = userID
	f.deleteMembershipN++
	return nil
}

func (f *fakeClerkTenantClient) RevokeOrganizationInvitation(_ context.Context, _ clerkRevokeOrganizationInvitationInput) error {
	return nil
}

func (f *fakeClerkTenantClient) CreateOrganizationMembership(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeClerkTenantClient) AcceptOrganizationInvitationTicket(_ context.Context, _ string) error {
	return nil
}

func (f *fakeClerkTenantClient) GetUserIDByEmail(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (f *fakeClerkTenantClient) UserHasOrganizationMembership(_ context.Context, _, _ string) (bool, error) {
	return f.membershipOK, nil
}

func (f *fakeClerkTenantClient) SetUserPassword(_ context.Context, _, _ string) error {
	return nil
}

func TestEnrichAuthenticatedClerkUserFetchesRealClerkProfile(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	clerk := &fakeClerkTenantClient{userProfile: clerkUserProfile{
		Email:     " devpablocristo@gmail.com ",
		FirstName: "Pablo",
		LastName:  "Cristo",
		ImageURL:  "https://img.test/avatar.png",
	}}
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = clerk

	user := store.enrichAuthenticatedClerkUser(context.Background(), clerkAuthenticatedUser{
		ExternalID: "user_3AXavi5Algpygf3F8NxLWf5r88I",
		Email:      placeholderClerkEmail("user_3AXavi5Algpygf3F8NxLWf5r88I"),
		Name:       placeholderClerkEmail("user_3AXavi5Algpygf3F8NxLWf5r88I"),
	})

	if clerk.getUserN != 1 {
		t.Fatalf("GetUser calls = %d, want 1", clerk.getUserN)
	}
	if user.Email != "devpablocristo@gmail.com" {
		t.Fatalf("email = %q, want real Clerk email", user.Email)
	}
	if user.Name != "Pablo Cristo" {
		t.Fatalf("name = %q, want Clerk display name", user.Name)
	}
	if user.AvatarURL == nil || *user.AvatarURL != "https://img.test/avatar.png" {
		t.Fatalf("avatar = %#v, want Clerk image", user.AvatarURL)
	}
}

func TestUpsertUserDoesNotOverwriteRealProfileWithPlaceholder(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	if _, err := store.upsertUserTx(ctx, db, "user_owner", "devpablocristo@gmail.com", "Pablo Cristo", nil); err != nil {
		t.Fatalf("initial upsertUserTx() error = %v", err)
	}
	row, err := store.upsertUserTx(ctx, db, "user_owner", placeholderClerkEmail("user_owner"), "", nil)
	if err != nil {
		t.Fatalf("placeholder upsertUserTx() error = %v", err)
	}
	if row.Email != "devpablocristo@gmail.com" {
		t.Fatalf("email = %q, want original real email", row.Email)
	}
	if row.Name != "Pablo Cristo" || row.GivenName != "Pablo" || row.FamilyName != "Cristo" {
		t.Fatalf("name fields = %q/%q/%q, want original profile", row.Name, row.GivenName, row.FamilyName)
	}
}

func TestUpsertUserRelinksVerifiedEmailWhenClerkUserIDChanges(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	original, err := store.upsertUserTx(ctx, db, "user_old", "tucbox@gmail.com", "Tuc Box", nil)
	if err != nil {
		t.Fatalf("initial upsertUserTx() error = %v", err)
	}
	updated, err := store.upsertUserTx(ctx, db, "user_new", "tucbox@gmail.com", "Tuc Box", nil)
	if err != nil {
		t.Fatalf("relink upsertUserTx() error = %v", err)
	}
	if updated.ID != original.ID {
		t.Fatalf("user id = %s, want existing id %s", updated.ID, original.ID)
	}
	if updated.ExternalID != "user_new" {
		t.Fatalf("external_id = %q, want user_new", updated.ExternalID)
	}
	var count int64
	if err := db.Model(&pymesUserRow{}).Where("email = ?", "tucbox@gmail.com").Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users with email = %d, want 1", count)
	}
}

func TestFindActiveMembershipRoleByExternalUserUsesLocalRole(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	user, err := store.upsertUserTx(ctx, db, "user_member", "member@bicimax.test", "Member", nil)
	if err != nil {
		t.Fatalf("upsertUserTx() error = %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, orgID, user.ID.String(), "member"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}

	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_member")
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

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
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
	if row.ID.String() != orgID {
		t.Fatalf("tenant id = %q, want %q", row.ID.String(), orgID)
	}
	if role != "owner" {
		t.Fatalf("role = %q, want owner", role)
	}
	if row.ClerkOrgID == nil || *row.ClerkOrgID != "org_medlab" {
		t.Fatalf("ClerkOrgID = %v, want org_medlab", row.ClerkOrgID)
	}
}

func TestCreateTenantWithClerkOrganizationCreatesTenantOrgAndOwner(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	clerk := &fakeClerkTenantClient{membershipOK: true}
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = clerk
	ctx := context.Background()

	orgID, clerkOrgID, _, _, _, err := store.CreateTenantWithClerkOrganization(ctx, "MedLab", "medlab", "", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithClerkOrganization() error = %v", err)
	}
	if clerk.createOrgN != 1 {
		t.Fatalf("CreateOrganization calls = %d, want 1", clerk.createOrgN)
	}
	if clerk.lastCreateOrg.Name != "MedLab" || clerk.lastCreateOrg.CreatedBy != "user_owner" {
		t.Fatalf("CreateOrganization input = %#v", clerk.lastCreateOrg)
	}
	if got := clerk.lastCreateOrg.PublicMetadata["pymes_tenant_slug"]; got != "medlab" {
		t.Fatalf("pymes_tenant_slug metadata = %v, want medlab", got)
	}
	if clerkOrgID != "org_medlab" {
		t.Fatalf("clerkOrgID = %q, want org_medlab", clerkOrgID)
	}
	var tenant pymesTenantRow
	if err := db.Where("id = ?", orgID).Take(&tenant).Error; err != nil {
		t.Fatalf("load tenant: %v", err)
	}
	if clerkTenantIDFromTenant(tenant) != "org_medlab" {
		t.Fatalf("tenant clerk org = %q, want org_medlab", clerkTenantIDFromTenant(tenant))
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_owner")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "owner" {
		t.Fatalf("owner membership = role %q ok %v, want owner true", role, ok)
	}
}

func TestTransferTenantOwnershipKeepsExactlyOneOwner(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	next, err := store.upsertUserTx(ctx, db, "user_next_owner", "next@bicimax.test", "Next Owner", nil)
	if err != nil {
		t.Fatalf("upsertUserTx() error = %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, orgID, next.ID.String(), "admin"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}

	if err := store.TransferTenantOwnership(ctx, orgID, "user_owner", next.ID.String()); err != nil {
		t.Fatalf("TransferTenantOwnership() error = %v", err)
	}

	tenantUUID := uuid.MustParse(orgID)
	var owners int64
	if err := db.Model(&pymesTenantMembershipRow{}).
		Where("org_id = ? AND role = 'owner' AND status = 'active'", tenantUUID).
		Count(&owners).Error; err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if owners != 1 {
		t.Fatalf("active owners = %d, want 1", owners)
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_next_owner")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "owner" {
		t.Fatalf("next owner role = %q ok=%v, want owner true", role, ok)
	}
}

func TestRemoveTenantMemberRemovesClerkOrganizationMembership(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	clerk := &fakeClerkTenantClient{}
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = clerk
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	user, err := store.upsertUserTx(ctx, db, "user_member", "member@medlab.test", "Member User", nil)
	if err != nil {
		t.Fatalf("upsertUserTx() error = %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, orgID, user.ID.String(), "member"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}

	if err := store.RemoveTenantMember(ctx, orgID, user.ID.String()); err != nil {
		t.Fatalf("RemoveTenantMember() error = %v", err)
	}

	if clerk.deleteMembershipN != 1 {
		t.Fatalf("DeleteOrganizationMembership calls = %d, want 1", clerk.deleteMembershipN)
	}
	if clerk.lastDeletedOrgMembershipID != "org_medlab" || clerk.lastDeletedOrgMembershipUserID != "user_member" {
		t.Fatalf("deleted membership = org %q user %q, want org_medlab/user_member", clerk.lastDeletedOrgMembershipID, clerk.lastDeletedOrgMembershipUserID)
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_member")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if ok || role != "" {
		t.Fatalf("active membership = role %q ok %v, want none", role, ok)
	}
}

func TestAcceptTenantInvitationCreatesMembershipForCorrectTenant(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
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
		OrgID:          uuid.MustParse(orgID),
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

	accepted, clerkTenantID, err := store.AcceptTenantInvitation(ctx, token, "", clerkAuthenticatedUser{
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
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_new")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "member" {
		t.Fatalf("accepted role = %q ok=%v, want member true", role, ok)
	}
}

func TestAcceptTenantInvitationRelinksExistingEmailAndReactivatesMembership(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	oldUser, err := store.upsertUserTx(ctx, db, "user_old", "tucbox@gmail.com", "Tuc Box", nil)
	if err != nil {
		t.Fatalf("upsert old user: %v", err)
	}
	if _, err := store.UpsertTenantMember(ctx, orgID, oldUser.ID.String(), "member"); err != nil {
		t.Fatalf("UpsertTenantMember() error = %v", err)
	}
	if err := store.RemoveTenantMember(ctx, orgID, oldUser.ID.String()); err != nil {
		t.Fatalf("RemoveTenantMember() error = %v", err)
	}
	owner, ok, err := store.LocalUserByExternalID(ctx, "user_owner")
	if err != nil || !ok {
		t.Fatalf("LocalUserByExternalID() = ok %v err %v", ok, err)
	}
	token := "invite-token-relink"
	now := time.Now().UTC()
	row := pymesTenantInvitationRow{
		ID:                uuid.New(),
		OrgID:          uuid.MustParse(orgID),
		EmailNormalized:   "tucbox@gmail.com",
		Role:              "admin",
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

	accepted, _, err := store.AcceptTenantInvitation(ctx, token, "", clerkAuthenticatedUser{
		ExternalID: "user_new",
		Email:      "tucbox@gmail.com",
		Name:       "Tuc Box",
	})
	if err != nil {
		t.Fatalf("AcceptTenantInvitation() error = %v", err)
	}
	if accepted.Status != "accepted" {
		t.Fatalf("accepted status = %q, want accepted", accepted.Status)
	}
	var user pymesUserRow
	if err := db.Where("email = ?", "tucbox@gmail.com").Take(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.ID != oldUser.ID || user.ExternalID != "user_new" {
		t.Fatalf("user = id %s external %q, want id %s external user_new", user.ID, user.ExternalID, oldUser.ID)
	}
	role, ok, err := store.FindActiveMembershipRoleByExternalUser(ctx, orgID, "user_new")
	if err != nil {
		t.Fatalf("FindActiveMembershipRoleByExternalUser() error = %v", err)
	}
	if !ok || role != "admin" {
		t.Fatalf("accepted role = %q ok=%v, want admin true", role, ok)
	}
}

func TestCreateTenantInvitationUsesTenantClerkOrganization(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	clerk := &fakeClerkTenantClient{}
	store.clerk = clerk
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}

	invite, err := store.CreateTenantInvitation(ctx, orgID, "user_owner", "admin@medlab.test", "admin")
	if err != nil {
		t.Fatalf("CreateTenantInvitation() error = %v", err)
	}
	if invite.Status != "pending" {
		t.Fatalf("invite status = %q, want pending", invite.Status)
	}
	if invite.ClerkInvitationID == nil || *invite.ClerkInvitationID != "clerk_invite_test" {
		t.Fatalf("ClerkInvitationID = %v, want clerk_invite_test", invite.ClerkInvitationID)
	}
	if clerk.createInviteN != 1 {
		t.Fatalf("CreateOrganizationInvitation calls = %d, want 1", clerk.createInviteN)
	}
	if clerk.lastInviteInput.OrganizationID != "org_medlab" {
		t.Fatalf("OrganizationID = %q, want org_medlab", clerk.lastInviteInput.OrganizationID)
	}
	if clerk.lastInviteInput.Email != "admin@medlab.test" {
		t.Fatalf("Email = %q", clerk.lastInviteInput.Email)
	}
	if clerk.lastInviteInput.Role != "org:member" {
		t.Fatalf("Role = %q, want org:member", clerk.lastInviteInput.Role)
	}
	if got := clerk.lastInviteInput.PublicMetadata["pymes_tenant_id"]; got != orgID {
		t.Fatalf("pymes_tenant_id metadata = %v, want %s", got, orgID)
	}
	if got := clerk.lastInviteInput.PublicMetadata["pymes_tenant_slug"]; got != "medlab" {
		t.Fatalf("pymes_tenant_slug metadata = %v, want medlab", got)
	}
	if got := clerk.lastInviteInput.PublicMetadata["pymes_role"]; got != "admin" {
		t.Fatalf("pymes_role metadata = %v, want admin", got)
	}
}

func TestPreviewTenantInvitationReturnsTenantDestinationWithoutAccepting(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}
	invite, err := store.CreateTenantInvitation(ctx, orgID, "user_owner", "tucbox@gmail.com", "member")
	if err != nil {
		t.Fatalf("CreateTenantInvitation() error = %v", err)
	}
	// El redirect_url ahora apunta al backend exchange endpoint (Phase 6.1):
	// `${publicBaseURL}/v1/tenant-invites/exchange?token=<token>`. El backend
	// procesa el ticket server-side cuando el SDK frontend no puede.
	rawRedirect := store.clerk.(*fakeClerkTenantClient).lastInviteInput.RedirectURL
	parsed, err := url.Parse(rawRedirect)
	if err != nil {
		t.Fatalf("parse redirect url %q: %v", rawRedirect, err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("redirect url = %q, missing token query param", rawRedirect)
	}

	preview, err := store.PreviewTenantInvitation(ctx, token)
	if err != nil {
		t.Fatalf("PreviewTenantInvitation() error = %v", err)
	}
	if preview.OrgID != orgID || preview.TenantSlug != "medlab" || preview.TenantName != "MedLab" {
		t.Fatalf("preview tenant = id %q slug %q name %q, want %q medlab MedLab", preview.OrgID, preview.TenantSlug, preview.TenantName, orgID)
	}
	if preview.Email != "tucbox@gmail.com" || preview.Role != "member" || preview.Status != "pending" {
		t.Fatalf("preview invite = email %q role %q status %q", preview.Email, preview.Role, preview.Status)
	}
	var row pymesTenantInvitationRow
	if err := db.Where("id = ?", invite.ID).Take(&row).Error; err != nil {
		t.Fatalf("load invite: %v", err)
	}
	if row.Status != "pending" || row.AcceptedAt != nil || row.AcceptedByUserID != nil {
		t.Fatalf("row after preview = status %q accepted_at %v accepted_by %v, want untouched pending", row.Status, row.AcceptedAt, row.AcceptedByUserID)
	}
}

func TestCreateTenantInvitationDoesNotLeavePendingInviteWhenClerkFails(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{createInviteErr: errors.New("clerk down")}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
	if err != nil {
		t.Fatalf("CreateTenantWithOwner() error = %v", err)
	}

	_, err = store.CreateTenantInvitation(ctx, orgID, "user_owner", "admin@medlab.test", "admin")
	if err == nil {
		t.Fatal("CreateTenantInvitation() error = nil, want clerk error")
	}
	var count int64
	if err := db.Model(&pymesTenantInvitationRow{}).
		Where("org_id = ? AND email_normalized = ? AND status = 'pending'", uuid.MustParse(orgID), "admin@medlab.test").
		Count(&count).Error; err != nil {
		t.Fatalf("count invites: %v", err)
	}
	if count != 0 {
		t.Fatalf("pending invites = %d, want 0", count)
	}
}

func TestAcceptTenantInvitationRejectsLocalOnlyPendingInvite(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
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
	token := "local-only-invite-token-test"
	now := time.Now().UTC()
	if err := db.Create(&pymesTenantInvitationRow{
		ID:              uuid.New(),
		OrgID:        uuid.MustParse(orgID),
		EmailNormalized: "new@medlab.test",
		Role:            "member",
		Status:          "pending",
		TokenHash:       hashInviteToken(token),
		InvitedByUserID: owner.ID,
		ExpiresAt:       now.Add(time.Hour),
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, _, err = store.AcceptTenantInvitation(ctx, token, "", clerkAuthenticatedUser{
		ExternalID: "user_new",
		Email:      "new@medlab.test",
		Name:       "New Member",
	})
	if !errors.Is(err, domainerr.Conflict("")) {
		t.Fatalf("AcceptTenantInvitation() error = %v, want conflict", err)
	}
}

func TestAcceptTenantInvitationCreatesMissingClerkMembership(t *testing.T) {
	// Cuando el user invitado todavía no tiene membership en Clerk org, el
	// store debe completar el flow server-side: revoke de la invitation
	// pendiente + create membership Clerk + create membership local. Esto es
	// el fallback que cierra el caso "user con sesión activa, SDK frontend
	// no procesó el ticket" sin abandonar la invitation en estado raro.
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	fakeClerk := &fakeClerkTenantClient{membershipOK: false}
	store.clerk = fakeClerk
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "MedLab", "medlab", "org_medlab", "user_owner", "owner@medlab.test", "Owner", nil)
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
	token := "pymes-membership-required-token-test"
	now := time.Now().UTC()
	if err := db.Create(&pymesTenantInvitationRow{
		ID:                uuid.New(),
		OrgID:          uuid.MustParse(orgID),
		EmailNormalized:   "new@medlab.test",
		Role:              "member",
		Status:            "pending",
		TokenHash:         hashInviteToken(token),
		ClerkInvitationID: stringPtr("clerk_invite_test"),
		InvitedByUserID:   owner.ID,
		ExpiresAt:         now.Add(time.Hour),
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create invite: %v", err)
	}

	invite, clerkOrgID, err := store.AcceptTenantInvitation(ctx, token, "", clerkAuthenticatedUser{
		ExternalID: "user_new",
		Email:      "new@medlab.test",
		Name:       "New Member",
	})
	if err != nil {
		t.Fatalf("AcceptTenantInvitation() error = %v, want nil (fallback creates membership)", err)
	}
	if invite.Status != "accepted" {
		t.Fatalf("invite.Status = %q, want accepted", invite.Status)
	}
	if clerkOrgID != "org_medlab" {
		t.Fatalf("clerkOrgID = %q, want org_medlab", clerkOrgID)
	}
}

func TestAcceptTenantInvitationRejectsEmailMismatch(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)
	store.clerk = &fakeClerkTenantClient{membershipOK: true}
	ctx := context.Background()

	orgID, _, _, _, err := store.CreateTenantWithOwner(ctx, "Bicimax", "bicimax", "org_bicimax", "user_owner", "owner@bicimax.test", "Owner", nil)
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
		ID:                uuid.New(),
		OrgID:          uuid.MustParse(orgID),
		EmailNormalized:   "expected@bicimax.test",
		Role:              "admin",
		Status:            "pending",
		TokenHash:         hashInviteToken(token),
		ClerkInvitationID: stringPtr("clerk_invite_test"),
		InvitedByUserID:   owner.ID,
		ExpiresAt:         now.Add(time.Hour),
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, _, err = store.AcceptTenantInvitation(ctx, token, "", clerkAuthenticatedUser{
		ExternalID: "user_wrong",
		Email:      "wrong@bicimax.test",
		Name:       "Wrong User",
	})
	if !errors.Is(err, domainerr.Forbidden("")) {
		t.Fatalf("AcceptTenantInvitation() error = %v, want forbidden", err)
	}
}
