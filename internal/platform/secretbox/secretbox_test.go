package secretbox

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, KeySize) }

func TestSealOpen_RoundTrip(t *testing.T) {
	box, err := New(testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("raw-token-value"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "raw-token") {
		t.Fatal("sealed value exposes plaintext")
	}
	got, err := box.Open(sealed)
	if err != nil || string(got) != "raw-token-value" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestSeal_UsesFreshNonce(t *testing.T) {
	box, _ := New(testKey(1))
	a, _ := box.Seal([]byte("same"))
	b, _ := box.Seal([]byte("same"))
	if a == b {
		t.Fatal("two seals of the same plaintext must differ")
	}
}

func TestOpen_RejectsTamperingWrongKeyAndGarbage(t *testing.T) {
	box, _ := New(testKey(1))
	other, _ := New(testKey(2))
	sealed, _ := box.Seal([]byte("secret"))

	if _, err := other.Open(sealed); !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("wrong key error = %v", err)
	}
	tampered := []byte(sealed)
	tampered[len(tampered)-1] ^= 'x'
	if _, err := box.Open(string(tampered)); !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("tampered error = %v", err)
	}
	for _, bad := range []string{"", "!!!", "AAAA", sealed[:10]} {
		if _, err := box.Open(bad); !errors.Is(err, ErrInvalidCiphertext) {
			t.Errorf("Open(%q) error = %v", bad, err)
		}
	}
}

func TestNew_RequiresExactKeySize(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := New(bytes.Repeat([]byte{1}, n)); err == nil {
			t.Errorf("key of %d bytes accepted", n)
		}
	}
}
