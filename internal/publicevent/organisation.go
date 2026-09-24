package publicevent

import (
	"errors"
	"strings"
)

// ErrInvalidOrganisationNumber is returned for values that are not a
// Swedish organisation number.
var ErrInvalidOrganisationNumber = errors.New("publicevent: invalid organisation number")

// NormalizeOrganisationNumber converts the written forms of a Swedish
// organisation number to the canonical ten digits and verifies the Luhn
// check digit.
//
// Accepted forms: "5594800418", "559480-0418", and the twelve-digit form
// with a 16 (organisations) or 19/20 (sole traders, whose number is a
// personal identity number) century prefix. Whitespace is ignored.
func NormalizeOrganisationNumber(raw string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == ' ' || r == '+':
			// separators
		default:
			return "", ErrInvalidOrganisationNumber
		}
	}
	digits := b.String()
	if len(digits) == 12 {
		switch digits[:2] {
		case "16", "19", "20":
			digits = digits[2:]
		default:
			return "", ErrInvalidOrganisationNumber
		}
	}
	if len(digits) != 10 || !luhnValid(digits) {
		return "", ErrInvalidOrganisationNumber
	}
	return digits, nil
}

// luhnValid checks the Luhn check digit used by Swedish organisation and
// personal identity numbers: from the left, every other digit starting with
// the first is doubled (digits of the product summed) and the total must be
// divisible by ten.
func luhnValid(digits string) bool {
	sum := 0
	for i, r := range digits {
		d := int(r - '0')
		if i%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}
