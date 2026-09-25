package arbetsmiljoverket

import (
	"errors"
	"strings"
	"testing"

	"github.com/sK4rdell/signal-engine/internal/publicevent"
)

func TestFeeds(t *testing.T) {
	if DefaultFeed.Name != "inspection-notices" || DefaultFeed.EventType != publicevent.EventTypeWorkEnvironmentInspectionNotice ||
		DefaultFeed.SubjectArea != "6.1" || DefaultFeed.DocumentType != "6.1-23" || DefaultFeed.DocumentTypeName != "Inspektionsmeddelande" {
		t.Errorf("default feed = %+v", DefaultFeed)
	}
	if f := FeedRecurringInspectionFailures; f.Name != "recurring-inspection-failures" || f.EventType != publicevent.EventTypeWorkEquipmentInspectionFailed ||
		f.SubjectArea != "6.1" || f.DocumentType != "6.1-49" || f.DocumentTypeName != "Intyg återkommande besiktning" || !f.OnePerCase {
		t.Errorf("certificate feed = %+v", f)
	}
	if DefaultFeed.OnePerCase {
		t.Error("inspection notices are not limited to one per case")
	}
	for _, f := range Feeds() {
		if err := f.validate(); err != nil {
			t.Errorf("%s: %v", f.Name, err)
		}
		got, err := FeedByName(f.Name)
		if err != nil || got.Name != f.Name || got.EventType != f.EventType {
			t.Errorf("FeedByName(%q) = %+v, %v", f.Name, got, err)
		}
	}
	_, err := FeedByName("sanction-orders")
	if !errors.Is(err, ErrUnknownFeed) || !strings.Contains(err.Error(), "inspection-notices") || !strings.Contains(err.Error(), "recurring-inspection-failures") {
		t.Errorf("unknown feed error = %v", err)
	}
	if err := (Feed{Name: "half"}).validate(); err == nil {
		t.Error("incomplete feed should not validate")
	}
	// The inspection-notice feed accepts every row of its document type.
	if ok, _ := FeedInspectionNotices.Accept(Document{CaseTitle: "Olycka 20260907 Fysiskt våld", Origin: "Utgående"}); !ok {
		t.Error("inspection notices should accept any title")
	}
}

func TestAcceptRecurringInspectionFailure(t *testing.T) {
	cases := []struct {
		name   string
		doc    Document
		accept bool
	}{
		{"failed vehicle lift", Document{Origin: "Inkommande", CaseTitle: "Återkommande besiktning - Fordonslyft flerpelarlyft"}, true},
		{"failed pressure vessel", Document{Origin: "Inkommande", CaseTitle: "Återkommande besiktning - Tryckluftbehållare"}, true},
		{"lower-case title as the source sometimes prints it", Document{Origin: "Inkommande", CaseTitle: "återkommande besiktning -Kompressor"}, true},
		{"surrounding whitespace", Document{Origin: " Inkommande ", CaseTitle: "  Återkommande besiktning - Mobilplattform "}, true},
		{"approved inspection sent for information", Document{Origin: "Inkommande", CaseTitle: "För kännedom - godkänd besiktning, fordonslyft flerpelarlyft"}, false},
		{"for-information prefix before the pattern", Document{Origin: "Inkommande", CaseTitle: "För kännedom - Återkommande besiktning - Port vertikal"}, false},
		{"inspection campaign case", Document{Origin: "Inkommande", CaseTitle: "Inspektion inom Myndighetsgemensamma kontroller"}, false},
		{"crusher notification case", Document{Origin: "Inkommande", CaseTitle: "Uppställning av krossverk - Gillstad"}, false},
		{"outgoing document", Document{Origin: "Utgående", CaseTitle: "Återkommande besiktning - Fordonskran"}, false},
		{"internally drawn up", Document{Origin: "Upprättad", CaseTitle: "Återkommande besiktning - Traverskran maskindriven"}, false},
		{"empty", Document{}, false},
	}
	for _, tc := range cases {
		ok, reason := FeedRecurringInspectionFailures.Accept(tc.doc)
		if ok != tc.accept {
			t.Errorf("%s: accept = %v (%s), want %v", tc.name, ok, reason, tc.accept)
		}
		if !ok && reason == "" {
			t.Errorf("%s: rejected without a reason", tc.name)
		}
	}
}
