package account_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"betemplate/internal/account"
	"betemplate/internal/auth"
	"betemplate/internal/platform/testutil/pgtest"
)

func TestRepository_AccountsAndMemberships(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	repo := account.NewRepository()
	users := auth.NewRepository()

	alice, err := users.CreateUser(ctx, pool, "alice@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := users.CreateUser(ctx, pool, "bob@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}

	acc, err := repo.Create(ctx, pool, "Alice's workspace")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if acc.ID == uuid.Nil || acc.Name != "Alice's workspace" || acc.CreatedAt.IsZero() {
		t.Errorf("account = %+v", acc)
	}

	m, err := repo.AddMembership(ctx, pool, acc.ID, alice.ID, account.RoleOwner)
	if err != nil {
		t.Fatalf("AddMembership: %v", err)
	}
	if m.Role != account.RoleOwner {
		t.Errorf("role = %q", m.Role)
	}

	_, err = repo.AddMembership(ctx, pool, acc.ID, alice.ID, account.RoleMember)
	if !errors.Is(err, account.ErrAlreadyMember) {
		t.Errorf("duplicate membership error = %v", err)
	}

	got, err := repo.GetMembership(ctx, pool, acc.ID, alice.ID)
	if err != nil || got.Role != account.RoleOwner {
		t.Errorf("GetMembership = %+v, %v", got, err)
	}
	_, err = repo.GetMembership(ctx, pool, acc.ID, bob.ID)
	if !errors.Is(err, account.ErrMembershipRequired) {
		t.Errorf("non-member GetMembership error = %v", err)
	}

	withRole, err := repo.GetForUser(ctx, pool, acc.ID, alice.ID)
	if err != nil || withRole.ID != acc.ID || withRole.Role != account.RoleOwner {
		t.Errorf("GetForUser = %+v, %v", withRole, err)
	}
	_, err = repo.GetForUser(ctx, pool, acc.ID, bob.ID)
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Errorf("non-member GetForUser error = %v (must not reveal existence)", err)
	}
	_, err = repo.GetForUser(ctx, pool, uuid.New(), alice.ID)
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Errorf("unknown account error = %v", err)
	}

	second, _ := repo.Create(ctx, pool, "Shared")
	if _, err := repo.AddMembership(ctx, pool, second.ID, alice.ID, account.RoleMember); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListForUser(ctx, pool, alice.ID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(list) != 2 || list[0].ID != acc.ID || list[1].Role != account.RoleMember {
		t.Errorf("ListForUser = %+v", list)
	}
	empty, err := repo.ListForUser(ctx, pool, bob.ID)
	if err != nil || len(empty) != 0 {
		t.Errorf("bob's accounts = %v, %v", empty, err)
	}

	// Invalid roles are refused by the database.
	if _, err := repo.AddMembership(ctx, pool, second.ID, bob.ID, account.Role("admin")); err == nil {
		t.Error("invalid role should be rejected by the check constraint")
	}
}
