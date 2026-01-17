package middleware

import "testing"

func TestRequestHashDeterministic(t *testing.T) {
	hash1 := RequestHash([]byte("payload"))
	hash2 := RequestHash([]byte("payload"))
	hash3 := RequestHash([]byte("other"))

	if hash1 != hash2 {
		t.Fatal("expected deterministic hash")
	}
	if hash1 == hash3 {
		t.Fatal("expected different hash for different payload")
	}
}
