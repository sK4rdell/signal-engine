package password

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format %q", hash)
	}
	ok, err := Verify(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("Verify correct = %v, %v", ok, err)
	}
	ok, err = Verify(hash, "correct horse battery stapl")
	if err != nil || ok {
		t.Fatalf("Verify wrong = %v, %v", ok, err)
	}
}

func TestHash_UsesFreshSalt(t *testing.T) {
	a, _ := Hash("correct horse battery staple")
	b, _ := Hash("correct horse battery staple")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestHash_EnforcesLength(t *testing.T) {
	if _, err := Hash("short"); err == nil {
		t.Error("short password should be rejected")
	}
	if _, err := Hash(strings.Repeat("x", MaxLength+1)); err == nil {
		t.Error("long password should be rejected")
	}
}

func TestVerify_RejectsMalformedHashes(t *testing.T) {
	for _, hash := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=1,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$!!!$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$",
	} {
		_, err := Verify(hash, "correct horse battery staple")
		if !errors.Is(err, ErrInvalidHash) {
			t.Errorf("Verify(%q) error = %v, want ErrInvalidHash", hash, err)
		}
	}
}

func TestVerify_UsesStoredParameters(t *testing.T) {
	weak := Params{MemoryKiB: 8 * 1024, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 16}
	hash, err := HashWithParams("correct horse battery staple", weak)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("Verify with stored params = %v, %v", ok, err)
	}
	if !NeedsRehash(hash) {
		t.Error("weak hash should need rehash")
	}
	strong, _ := Hash("correct horse battery staple")
	if NeedsRehash(strong) {
		t.Error("default hash should not need rehash")
	}
	if !NeedsRehash("garbage") {
		t.Error("malformed hash should need rehash")
	}
}

func TestDummyHash_IsValid(t *testing.T) {
	ok, err := Verify(DummyHash, "anything at all here")
	if err != nil {
		t.Fatalf("DummyHash must be well-formed: %v", err)
	}
	if ok {
		t.Fatal("DummyHash must not match arbitrary input")
	}
}
