package subtpl

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/crypto-vectors.json comes from upstream's helpers
// (scripts/subscription-diff/crypto-vectors.mjs).
func TestKeyDerivationMatchesUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/crypto-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors map[string][]struct {
		In  string  `json:"in"`
		Out *string `json:"out"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	derive := map[string]func(string) (string, error){"x25519": x25519PublicKey, "mldsa65": mldsa65PublicKey, "encryption": encryptionFromDecryption}
	for name, cases := range vectors {
		for _, c := range cases {
			got, err := derive[name](c.In)
			if c.Out == nil {
				if err == nil {
					t.Errorf("%s(%q) = %q, want error", name, c.In, got)
				}
				continue
			}
			if err != nil || got != *c.Out {
				t.Errorf("%s(%q) = %q, %v; want %q", name, c.In, got, err, *c.Out)
			}
		}
	}
}
