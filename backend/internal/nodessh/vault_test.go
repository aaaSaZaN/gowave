package nodessh

import (
	"encoding/base64"
	"github.com/cloudflare/circl/oprf"
	"testing"
)

// Generated with the exact @noble/curves and @noble/hashes implementation used
// by the pinned browser vault, executed with Bun. This detects key-derivation
// and OPRF ciphersuite changes that would make existing vaults unreadable.
func TestBrowserVaultCompatibility(t *testing.T) {
	blinded, _ := base64.StdEncoding.DecodeString("4vKuCmq8TnGohKlhxQBRX1jjC2qlgt2NtqZZReCNLXY=")
	element := oprf.SuiteRistretto255.Group().NewElement()
	if err := element.UnmarshalBinary(blinded); err != nil {
		t.Fatal(err)
	}
	server, err := vaultServer("compatibility-test-secret-at-least-32-characters")
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := server.Evaluate(&oprf.EvaluationRequest{Elements: []oprf.Blinded{element}})
	if err != nil {
		t.Fatal(err)
	}
	value, err := evaluation.Elements[0].MarshalBinaryCompress()
	if err != nil {
		t.Fatal(err)
	}
	if encoded := base64.StdEncoding.EncodeToString(value); encoded != "/FdUoNwpTDUedJiFjkjZl+8DJZBlL+imXghqsb5HzlQ=" {
		t.Fatalf("browser vault protocol changed: %s", encoded)
	}
}
