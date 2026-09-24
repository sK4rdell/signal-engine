package example_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/example"
	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
)

func base(accountID uuid.UUID) string {
	return "/v1/accounts/" + accountID.String() + "/examples"
}

func create(t *testing.T, client *testutil.Client, accountID uuid.UUID, title string) example.Response {
	t.Helper()
	var res example.Response
	client.PostJSON(base(accountID), map[string]string{"title": title, "note": "n"}).AssertStatus(t, http.StatusCreated).DecodeJSON(t, &res)
	return res
}

func TestExamples_CRUD(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	client := app.AuthenticatedClient(user)

	created := create(t, client, user.AccountID, "First")
	if created.ID == uuid.Nil || created.AccountID != user.AccountID || created.Title != "First" || created.Note != "n" {
		t.Errorf("created = %+v", created)
	}

	var got example.Response
	client.Get(base(user.AccountID)+"/"+created.ID.String()).AssertStatus(t, http.StatusOK).DecodeJSON(t, &got)
	if got.ID != created.ID {
		t.Errorf("got = %+v", got)
	}

	// PATCH: absent fields are untouched, explicit empty strings are applied.
	var updated example.Response
	client.PatchJSON(base(user.AccountID)+"/"+created.ID.String(), map[string]any{"note": ""}).AssertStatus(t, http.StatusOK).DecodeJSON(t, &updated)
	if updated.Title != "First" || updated.Note != "" {
		t.Errorf("patched = %+v", updated)
	}
	client.PatchJSON(base(user.AccountID)+"/"+created.ID.String(), map[string]any{"title": "Renamed"}).AssertStatus(t, http.StatusOK).DecodeJSON(t, &updated)
	if updated.Title != "Renamed" || updated.Note != "" || !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("patched = %+v", updated)
	}
	client.PatchJSON(base(user.AccountID)+"/"+created.ID.String(), map[string]any{"title": ""}).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)

	client.Delete(base(user.AccountID)+"/"+created.ID.String()).AssertStatus(t, http.StatusNoContent)
	client.Get(base(user.AccountID)+"/"+created.ID.String()).AssertError(t, http.StatusNotFound, example.CodeNotFound)
	client.Delete(base(user.AccountID)+"/"+created.ID.String()).AssertError(t, http.StatusNotFound, example.CodeNotFound)
}

func TestExamples_Validation(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	client := app.AuthenticatedClient(user)

	e := client.PostJSON(base(user.AccountID), map[string]string{"note": "x"}).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
	if e.Fields["title"] != "is required" {
		t.Errorf("fields = %v", e.Fields)
	}
	client.PostJSON(base(user.AccountID), `{"title":"x","extra":1}`).AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.PostJSON(base(user.AccountID), nil).AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.Get(base(user.AccountID)+"/not-a-uuid").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.Get(base(user.AccountID)+"?limit=101").AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
	client.Get(base(user.AccountID)+"?limit=abc").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.Get(base(user.AccountID)+"?cursor=%21%21").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

func TestExamples_RequireAuthenticationAndMembership(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.CreateUser()
	bob := app.CreateUser()
	shared := app.CreateAccount("Shared")
	app.AddMembership(alice, shared.ID, account.RoleMember)

	app.Client().Get(base(alice.AccountID)).AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)

	bobClient := app.AuthenticatedClient(bob)
	bobClient.Get(base(alice.AccountID)).AssertError(t, http.StatusForbidden, account.CodeMembershipRequired)
	bobClient.PostJSON(base(shared.ID), map[string]string{"title": "x"}).AssertError(t, http.StatusForbidden, account.CodeMembershipRequired)
	bobClient.Get("/v1/accounts/not-a-uuid/examples").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)

	aliceClient := app.AuthenticatedClient(alice)
	create(t, aliceClient, shared.ID, "in shared")
	aliceClient.Get(base(shared.ID)).AssertStatus(t, http.StatusOK)
}

func TestExamples_CrossAccountIsolation(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.CreateUser()
	bob := app.CreateUser()
	aliceClient := app.AuthenticatedClient(alice)
	bobClient := app.AuthenticatedClient(bob)

	secret := create(t, aliceClient, alice.AccountID, "Alice's")
	create(t, bobClient, bob.AccountID, "Bob's")

	// Bob addresses Alice's example through his own account: tenant-safe 404.
	path := base(bob.AccountID) + "/" + secret.ID.String()
	bobClient.Get(path).AssertError(t, http.StatusNotFound, example.CodeNotFound)
	bobClient.PatchJSON(path, map[string]string{"title": "pwned"}).AssertError(t, http.StatusNotFound, example.CodeNotFound)
	bobClient.Delete(path).AssertError(t, http.StatusNotFound, example.CodeNotFound)

	var list example.ListResponse
	bobClient.Get(base(bob.AccountID)).AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
	if len(list.Items) != 1 || list.Items[0].Title != "Bob's" {
		t.Errorf("bob sees %+v", list.Items)
	}

	// Alice's example is untouched.
	var got example.Response
	aliceClient.Get(base(alice.AccountID)+"/"+secret.ID.String()).AssertStatus(t, http.StatusOK).DecodeJSON(t, &got)
	if got.Title != "Alice's" {
		t.Errorf("alice's example = %+v", got)
	}
}

func TestExamples_CursorPaginationIsDeterministic(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	client := app.AuthenticatedClient(user)

	for i := 0; i < 7; i++ {
		create(t, client, user.AccountID, fmt.Sprintf("item %d", i))
	}

	var seen []string
	cursor := ""
	pages := 0
	for {
		path := base(user.AccountID) + "?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page example.ListResponse
		client.Get(path).AssertStatus(t, http.StatusOK).DecodeJSON(t, &page)
		pages++
		for _, item := range page.Items {
			seen = append(seen, item.Title)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}
	if pages != 3 || len(seen) != 7 {
		t.Fatalf("pages = %d, items = %d", pages, len(seen))
	}
	// Newest first, no duplicates, no gaps.
	for i, title := range seen {
		if want := fmt.Sprintf("item %d", 6-i); title != want {
			t.Errorf("seen[%d] = %q, want %q", i, title, want)
		}
	}

	var all example.ListResponse
	client.Get(base(user.AccountID)).AssertStatus(t, http.StatusOK).DecodeJSON(t, &all)
	if len(all.Items) != 7 || all.NextCursor != nil {
		t.Errorf("default page = %d items, cursor %v", len(all.Items), all.NextCursor)
	}
}
