package publicevent

import "testing"

func TestNormalizeOrganisationNumber(t *testing.T) {
	valid := map[string]string{
		"5594800418":   "5594800418", // aktiebolag as published by Arbetsmiljöverket
		"559480-0418":  "5594800418",
		"165594800418": "5594800418", // twelve-digit form with organisation prefix
		"2120001124":   "2120001124", // municipality
		"212000-1124":  "2120001124",
		" 5562802115 ": "5562802115",
		"5565371662":   "5565371662",
		"195401019996": "5401019996", // sole trader: personal identity number form (fabricated)
		"2021-00-0001": "2021000001",
	}
	for in, want := range valid {
		got, err := NormalizeOrganisationNumber(in)
		if err != nil || got != want {
			t.Errorf("NormalizeOrganisationNumber(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	invalid := []string{
		"",
		"5594800419",   // wrong check digit
		"559480041",    // nine digits
		"55948004181",  // eleven digits
		"175594800418", // unknown century prefix
		"559480O418",   // letter O
		"MITTEN MACK AB",
		"5594800418x",
	}
	for _, in := range invalid {
		if got, err := NormalizeOrganisationNumber(in); err == nil {
			t.Errorf("NormalizeOrganisationNumber(%q) = %q, want error", in, got)
		}
	}
}
