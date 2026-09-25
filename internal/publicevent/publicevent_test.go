package publicevent_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
)

const base = "/v1/public-events"

type seedEvent struct {
	id           string
	occurredOn   string
	orgNr        string
	orgName      string
	cfar         string
	workplace    string
	title        string
	payloadState string
}

// seed records an observation and derives its event, the way an ingester does.
func seed(t *testing.T, app *testutil.App, s seedEvent) publicevent.PublicEvent {
	t.Helper()
	ctx := context.Background()
	repo := publicevent.NewRepository()
	obs, _, err := repo.RecordObservation(ctx, app.Pool, publicevent.NewObservation{
		Source:         publicevent.SourceArbetsmiljoverket,
		SourceRecordID: s.id,
		SourceURL:      "https://www.av.se/diarium?p=1",
		Payload:        map[string]string{"document_number": s.id, "case_status": s.payloadState},
		Raw:            "<li>" + s.id + "</li>",
	})
	if err != nil {
		t.Fatal(err)
	}
	on, err := time.Parse("2006-01-02", s.occurredOn)
	if err != nil {
		t.Fatal(err)
	}
	e, _, err := repo.UpsertEvent(ctx, app.Pool, publicevent.NewEvent{
		Source:             publicevent.SourceArbetsmiljoverket,
		SourceEventID:      s.id,
		EventType:          publicevent.EventTypeWorkEnvironmentInspectionNotice,
		OccurredOn:         on,
		Title:              s.title,
		OrganisationNumber: s.orgNr,
		OrganisationName:   s.orgName,
		WorkplaceCFAR:      s.cfar,
		WorkplaceName:      s.workplace,
		SourceURL:          "https://www.av.se/diarium/Case/?id=" + strings.SplitN(s.id, "-", 2)[0],
		ObservationID:      obs.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestPublicEvents_RequireAuthentication(t *testing.T) {
	app := testutil.NewApp(t)
	app.Client().Get(base).AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	app.Client().Get(base+"/"+uuid.NewString()).AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
}

func TestPublicEvents_ListAndGetWithProvenance(t *testing.T) {
	app := testutil.NewApp(t)
	client := app.AuthenticatedClient(app.CreateUser())

	full := seed(t, app, seedEvent{id: "2026/060943-2", occurredOn: "2026-09-23", orgNr: "5594800418", orgName: "MITTEN MACK AB", cfar: "71466304", workplace: "MITTEN MACK AB", title: "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet", payloadState: "Pågående"})
	seed(t, app, seedEvent{id: "2026/058034-4", occurredOn: "2026-09-22", cfar: "73278392", title: "Inspektion inom Övrig tillsyn", payloadState: "Pågående"})
	seed(t, app, seedEvent{id: "2026/061028-2", occurredOn: "2026-09-21", orgNr: "5562802115", orgName: "RUSTA AB (PUBL)", cfar: "72896509", workplace: "RUSTA HÖÖR", title: "Inspektion inom Fortlöpande tillsyn Fall från samma nivå", payloadState: "Avslutat"})

	var list publicevent.ListResponse
	client.Get(base).AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
	if len(list.Items) != 3 || list.NextCursor != nil {
		t.Fatalf("list = %+v", list)
	}
	// Newest first.
	if list.Items[0].Source.SourceEventID != "2026/060943-2" || list.Items[1].Source.SourceEventID != "2026/058034-4" || list.Items[2].Source.SourceEventID != "2026/061028-2" {
		t.Errorf("order = %s, %s, %s", list.Items[0].Source.SourceEventID, list.Items[1].Source.SourceEventID, list.Items[2].Source.SourceEventID)
	}

	got := list.Items[0]
	if got.ID != full.ID || got.EventType != publicevent.EventTypeWorkEnvironmentInspectionNotice || got.OccurredOn != "2026-09-23" ||
		got.Title != "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet" ||
		got.Organisation == nil || got.Organisation.OrganisationNumber == nil || *got.Organisation.OrganisationNumber != "5594800418" || got.Organisation.Name != "MITTEN MACK AB" ||
		got.Workplace == nil || got.Workplace.CFAR == nil || *got.Workplace.CFAR != "71466304" || got.Workplace.Name != "MITTEN MACK AB" ||
		got.Source.Name != "arbetsmiljoverket" || got.Source.SourceEventID != "2026/060943-2" || got.Source.URL != "https://www.av.se/diarium/Case/?id=2026/060943" ||
		got.Source.ObservationID != full.ObservationID || got.Source.FirstObservedAt.IsZero() ||
		!strings.Contains(string(got.Source.Record), `"case_status":"Pågående"`) {
		t.Errorf("item = %+v", got)
	}
	noOrg := list.Items[1]
	if noOrg.Organisation != nil || noOrg.Workplace == nil || noOrg.Workplace.CFAR == nil || *noOrg.Workplace.CFAR != "73278392" || noOrg.Workplace.Name != "" {
		t.Errorf("item without organisation = %+v", noOrg)
	}

	// Raw JSON shape: snake_case, null organisation, record embedded.
	res := client.Get(base+"/"+full.ID.String()).AssertStatus(t, http.StatusOK)
	for _, want := range []string{`"event_type":"WORK_ENVIRONMENT_INSPECTION_NOTICE"`, `"occurred_on":"2026-09-23"`, `"organisation_number":"5594800418"`, `"cfar":"71466304"`, `"source":{"name":"arbetsmiljoverket"`, `"record":{`} {
		if !strings.Contains(string(res.Body), want) {
			t.Errorf("body lacks %s:\n%s", want, res.Body)
		}
	}
	res = client.Get(base+"/"+noOrg.ID.String()).AssertStatus(t, http.StatusOK)
	if !strings.Contains(string(res.Body), `"organisation":null`) {
		t.Errorf("body should have null organisation:\n%s", res.Body)
	}

	client.Get(base+"/"+uuid.NewString()).AssertError(t, http.StatusNotFound, publicevent.CodeNotFound)
	client.Get(base+"/not-a-uuid").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

func TestPublicEvents_Filters(t *testing.T) {
	app := testutil.NewApp(t)
	client := app.AuthenticatedClient(app.CreateUser())

	seed(t, app, seedEvent{id: "2026/000001-1", occurredOn: "2026-09-21", orgNr: "5594800418", orgName: "A", title: "a"})
	seed(t, app, seedEvent{id: "2026/000002-1", occurredOn: "2026-09-22", orgNr: "5594800418", orgName: "A", title: "b"})
	seed(t, app, seedEvent{id: "2026/000003-1", occurredOn: "2026-09-23", orgNr: "5562802115", orgName: "B", title: "c"})

	ids := func(path string) []string {
		t.Helper()
		var list publicevent.ListResponse
		client.Get(path).AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
		out := make([]string, 0, len(list.Items))
		for _, item := range list.Items {
			out = append(out, item.Source.SourceEventID)
		}
		return out
	}
	equal := func(got []string, want ...string) bool { return strings.Join(got, ",") == strings.Join(want, ",") }

	if got := ids(base + "?from=2026-09-22"); !equal(got, "2026/000003-1", "2026/000002-1") {
		t.Errorf("from = %v", got)
	}
	if got := ids(base + "?to=2026-09-22"); !equal(got, "2026/000002-1", "2026/000001-1") {
		t.Errorf("to = %v", got)
	}
	if got := ids(base + "?from=2026-09-22&to=2026-09-22"); !equal(got, "2026/000002-1") {
		t.Errorf("single day = %v", got)
	}
	if got := ids(base + "?organisation_number=559480-0418"); !equal(got, "2026/000002-1", "2026/000001-1") {
		t.Errorf("organisation (hyphenated) = %v", got)
	}
	if got := ids(base + "?organisation_number=5562802115&from=2026-09-23"); !equal(got, "2026/000003-1") {
		t.Errorf("organisation + from = %v", got)
	}
	if got := ids(base + "?source=arbetsmiljoverket&event_type=WORK_ENVIRONMENT_INSPECTION_NOTICE"); len(got) != 3 {
		t.Errorf("source + type = %v", got)
	}
	if got := ids(base + "?event_type=WORK_EQUIPMENT_INSPECTION_FAILED"); len(got) != 0 {
		t.Errorf("failed-inspection type must be accepted by the filter and match nothing here: %v", got)
	}

	for path, field := range map[string]string{
		base + "?event_type=SOMETHING_ELSE":      "event_type",
		base + "?source=bolagsverket":            "source",
		base + "?from=2026-9-1":                  "from",
		base + "?to=yesterday":                   "to",
		base + "?organisation_number=1234567890": "organisation_number",
		base + "?organisation_number=MITTEN":     "organisation_number",
		base + "?from=2026-09-23&to=2026-09-22":  "to",
		base + "?limit=101":                      "limit",
	} {
		e := client.Get(path).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
		if _, ok := e.Fields[field]; !ok {
			t.Errorf("%s: fields = %v, want %s", path, e.Fields, field)
		}
	}
	client.Get(base+"?limit=abc").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.Get(base+"?cursor=%21%21").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
	client.Get(base+"?from=2026-09-22&from=2026-09-23").AssertError(t, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

func TestPublicEvents_CursorPaginationIsDeterministic(t *testing.T) {
	app := testutil.NewApp(t)
	client := app.AuthenticatedClient(app.CreateUser())

	// Two events share a date so the cursor must also order by id.
	dates := []string{"2026-09-19", "2026-09-20", "2026-09-21", "2026-09-21", "2026-09-22", "2026-09-23", "2026-09-23"}
	for i, d := range dates {
		seed(t, app, seedEvent{id: fmt.Sprintf("2026/%06d-1", i+1), occurredOn: d, title: fmt.Sprintf("event %d", i)})
	}

	var seen []string
	cursor := ""
	pages := 0
	for {
		path := base + "?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page publicevent.ListResponse
		client.Get(path).AssertStatus(t, http.StatusOK).DecodeJSON(t, &page)
		pages++
		for _, item := range page.Items {
			seen = append(seen, item.Source.SourceEventID+"@"+item.OccurredOn)
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
		t.Fatalf("pages = %d, items = %d: %v", pages, len(seen), seen)
	}
	unique := map[string]bool{}
	for i, s := range seen {
		if unique[s] {
			t.Errorf("duplicate %s", s)
		}
		unique[s] = true
		if i > 0 && strings.SplitN(seen[i-1], "@", 2)[1] < strings.SplitN(s, "@", 2)[1] {
			t.Errorf("not newest first: %v", seen)
		}
	}
}

// seedProperty records an observation and derives a property event, the
// way the Stockholm ingester does.
func seedProperty(t *testing.T, app *testutil.App, id, occurredOn, designation, address, title string) publicevent.PublicEvent {
	t.Helper()
	ctx := context.Background()
	repo := publicevent.NewRepository()
	obs, _, err := repo.RecordObservation(ctx, app.Pool, publicevent.NewObservation{
		Source:         publicevent.SourceStockholm,
		SourceRecordID: strings.SplitN(id, ":", 2)[0],
		SourceURL:      "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/" + strings.SplitN(id, ":", 2)[0] + "?dataSource=Active",
		Payload:        map[string]string{"recno": strings.SplitN(id, ":", 2)[0], "district": "Östermalm", "closed_on": ""},
		Raw:            "<html>" + id + "</html>",
	})
	if err != nil {
		t.Fatal(err)
	}
	on, _ := time.Parse("2006-01-02", occurredOn)
	e, _, err := repo.UpsertEvent(ctx, app.Pool, publicevent.NewEvent{
		Source:                   publicevent.SourceStockholm,
		SourceEventID:            id,
		EventType:                publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed,
		OccurredOn:               on,
		Title:                    title,
		PropertyMunicipalityCode: "0180",
		PropertyDesignation:      designation,
		PropertyAddress:          address,
		SourceURL:                "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/" + strings.SplitN(id, ":", 2)[0] + "?dataSource=Active",
		ObservationID:            obs.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestPublicEvents_PropertyEvents(t *testing.T) {
	app := testutil.NewApp(t)
	client := app.AuthenticatedClient(app.CreateUser())
	ctx := context.Background()
	repo := publicevent.NewRepository()

	withAddress := seedProperty(t, app, "1327421:0f1e2d3c4b5a69788796a5b4c3d2e1f0", "2026-09-23", "Kronkvarnen 39", "Artillerigatan 48", "Intyg återkommande besiktning, ej godkänt, 53119")
	withoutAddress := seedProperty(t, app, "1327431:1f1e2d3c4b5a69788796a5b4c3d2e1f0", "2026-09-22", "Riksby 1:47", "", "Intyg återkommande besiktning, ej godkänt, D1618461")
	old := seed(t, app, seedEvent{id: "2026/060943-2", occurredOn: "2026-09-21", orgNr: "5594800418", orgName: "MITTEN MACK AB", cfar: "71466304", workplace: "MITTEN MACK AB", title: "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet", payloadState: "Pågående"})

	if withAddress.PropertyMunicipalityCode != "0180" || withAddress.PropertyDesignation != "Kronkvarnen 39" || withAddress.PropertyAddress != "Artillerigatan 48" ||
		withoutAddress.PropertyAddress != "" || withoutAddress.PropertyDesignation != "Riksby 1:47" || old.PropertyDesignation != "" || old.PropertyMunicipalityCode != "" {
		t.Errorf("stored events = %+v, %+v, %+v", withAddress, withoutAddress, old)
	}

	var list publicevent.ListResponse
	client.Get(base+"?source=stockholm&event_type=MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED").AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
	if len(list.Items) != 2 {
		t.Fatalf("list = %+v", list)
	}
	got := list.Items[0]
	if got.ID != withAddress.ID || got.EventType != publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed || got.Organisation != nil || got.Workplace != nil ||
		got.Property == nil || got.Property.MunicipalityCode != "0180" || got.Property.Designation != "Kronkvarnen 39" || got.Property.Address == nil || *got.Property.Address != "Artillerigatan 48" ||
		got.Source.Name != "stockholm" || got.Source.SourceEventID != "1327421:0f1e2d3c4b5a69788796a5b4c3d2e1f0" || !strings.Contains(string(got.Source.Record), `"district":"Östermalm"`) {
		t.Errorf("property item = %+v", got)
	}
	if got := list.Items[1]; got.Property == nil || got.Property.Address != nil || got.Property.Designation != "Riksby 1:47" {
		t.Errorf("item without address = %+v", got)
	}

	// Raw JSON: property object with null address, null organisation and workplace.
	res := client.Get(base+"/"+withoutAddress.ID.String()).AssertStatus(t, http.StatusOK)
	for _, want := range []string{`"event_type":"MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED"`, `"organisation":null`, `"workplace":null`, `"property":{"municipality_code":"0180","designation":"Riksby 1:47","address":null}`, `"source":{"name":"stockholm"`} {
		if !strings.Contains(string(res.Body), want) {
			t.Errorf("body lacks %s:\n%s", want, res.Body)
		}
	}
	res = client.Get(base+"/"+withAddress.ID.String()).AssertStatus(t, http.StatusOK)
	if !strings.Contains(string(res.Body), `"property":{"municipality_code":"0180","designation":"Kronkvarnen 39","address":"Artillerigatan 48"}`) {
		t.Errorf("body lacks property with address:\n%s", res.Body)
	}
	// Existing organisation/workplace events keep their shape with a null property.
	res = client.Get(base+"/"+old.ID.String()).AssertStatus(t, http.StatusOK)
	if !strings.Contains(string(res.Body), `"property":null`) || !strings.Contains(string(res.Body), `"organisation_number":"5594800418"`) {
		t.Errorf("old event body:\n%s", res.Body)
	}
	if got := ids(t, client, base+"?source=arbetsmiljoverket"); len(got) != 1 {
		t.Errorf("source filter = %v", got)
	}

	// Repository refuses half a property or an address without one.
	obs, _ := repo.ListObservations(ctx, app.Pool, publicevent.SourceStockholm, "1327421")
	for name, in := range map[string]publicevent.NewEvent{
		"designation without municipality": {Source: "stockholm", SourceEventID: "x:1", EventType: publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed, OccurredOn: withAddress.OccurredOn, Title: "t", PropertyDesignation: "Aida 3", ObservationID: obs[0].ID},
		"municipality without designation": {Source: "stockholm", SourceEventID: "x:2", EventType: publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed, OccurredOn: withAddress.OccurredOn, Title: "t", PropertyMunicipalityCode: "0180", ObservationID: obs[0].ID},
		"address without property":         {Source: "stockholm", SourceEventID: "x:3", EventType: publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed, OccurredOn: withAddress.OccurredOn, Title: "t", PropertyAddress: "Gatan 1", ObservationID: obs[0].ID},
	} {
		if _, _, err := repo.UpsertEvent(ctx, app.Pool, in); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// ids lists the source event ids a listing path returns.
func ids(t *testing.T, client *testutil.Client, path string) []string {
	t.Helper()
	var list publicevent.ListResponse
	client.Get(path).AssertStatus(t, http.StatusOK).DecodeJSON(t, &list)
	out := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		out = append(out, item.Source.SourceEventID)
	}
	return out
}
