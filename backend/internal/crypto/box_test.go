package crypto

import "testing"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	box, err := New("a-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain := `{"refresh_token":"super-secret-token","client_id":"abc"}`
	enc, err := box.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := box.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, plain)
	}
}

// Legacy plaintext (no prefix) must pass through unchanged, so accounts saved
// before encryption was enabled keep working.
func TestDecryptLegacyPlaintext(t *testing.T) {
	box, _ := New("k")
	plain := `{"token":"legacy"}`
	got, err := box.Decrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("legacy plaintext changed: got %q", got)
	}
}

// A value encrypted with one key must not decrypt with another.
func TestDecryptWrongKeyFails(t *testing.T) {
	a, _ := New("key-a")
	b, _ := New("key-b")
	enc, _ := a.Encrypt("secret")
	if _, err := b.Decrypt(enc); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}
