package server

import (
	"testing"

	"github.com/google/uuid"
)

func TestMissingComputedProfileReturnsNotFound(t *testing.T) {
	e := newProbeEnv(t)
	status, body := e.do("GET", "/api/config-profiles/"+uuid.NewString()+"/computed-config", nil)
	if status != 404 {
		t.Fatalf("missing computed config: %d %s", status, body)
	}
}
