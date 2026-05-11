package main

import (
	"path/filepath"
	"testing"
)

func newTestAuthStore(t *testing.T) *authStore {
	t.Helper()
	t.Setenv("REMORA_AUTH_DB", filepath.Join(t.TempDir(), "auth.db"))
	t.Setenv("REMORA_BOOTSTRAP_EMAIL", "")
	t.Setenv("REMORA_BOOTSTRAP_PASSWORD", "")
	s, err := openAuthStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.db.Close() })
	return s
}

func TestAuthStoreLimitsUserToOneBusiness(t *testing.T) {
	s := newTestAuthStore(t)
	user, err := s.createUser("one-business@example.com", "password123", "One", "user")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := s.createBusiness(user.ID, "First", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.createBusiness(user.ID, "Second", "", ""); err == nil {
		t.Fatal("expected second business creation to fail")
	}
	if err := s.upsertMembership(user.ID, first.ID, "admin", "admin", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.upsertMembership(user.ID, "other-business", "staff", "staff", nil); err == nil {
		t.Fatal("expected membership in another business to fail")
	}
}

func TestRemoraTeamInviteUpdatesGlobalRoleOnly(t *testing.T) {
	s := newTestAuthStore(t)
	admin, err := s.createUser("admin@example.com", "password123", "Admin", "remora_admin")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.createUser("staff@example.com", "password123", "Staff", "user")
	if err != nil {
		t.Fatal(err)
	}
	inv, code, err := s.createRemoraTeamInvite(admin.ID, "remora_staff")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.acceptRemoraTeamInvite(user.ID, inv.Token, code)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "remora_staff" {
		t.Fatalf("role = %q", updated.Role)
	}
	memberships, err := s.memberships(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 0 {
		t.Fatalf("memberships = %d, want 0", len(memberships))
	}
}
