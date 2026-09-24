package account_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
)

func TestAccounts_ListAndGetAreScopedToMembership(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.CreateUser()
	bob := app.CreateUser()
	shared := app.CreateAccount("Shared")
	app.AddMembership(alice, shared.ID, account.RoleMember)

	client := app.AuthenticatedClient(alice)

	var list account.ListResponse
	client.Get("/v1/accounts").AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
	if len(list.Items) != 2 || list.Items[0].ID != alice.AccountID || list.Items[0].Role != account.RoleOwner || list.Items[1].Role != account.RoleMember {
		t.Errorf("items = %+v", list.Items)
	}

	var got account.AccountResponse
	client.Get("/v1/accounts/"+shared.ID.String()).AssertStatus(t, http.StatusOK).DecodeJSON(t, &got)
	if got.ID != shared.ID || got.Name != "Shared" {
		t.Errorf("account = %+v", got)
	}

	// Bob's personal account is invisible to Alice.
	client.Get("/v1/accounts/"+bob.AccountID.String()).AssertError(t, http.StatusNotFound, account.CodeAccountNotFound)
	client.Get("/v1/accounts/"+uuid.NewString()).AssertError(t, http.StatusNotFound, account.CodeAccountNotFound)
	client.Get("/v1/accounts/not-a-uuid").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)

	app.Client().Get("/v1/accounts").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
}
