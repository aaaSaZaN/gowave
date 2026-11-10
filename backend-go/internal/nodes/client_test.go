package nodes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"
)

// A program on NODE_PORT that is not this panel's node used to surface only
// as "certificate signed by unknown authority".
func TestCheckHealthExplainsForeignCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "panel-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewNodeClient(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	stranger := httptest.NewTLSServer(http.NotFoundHandler())
	defer stranger.Close()
	host, portText, _ := net.SplitHostPort(stranger.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)

	result, err := client.CheckHealth(&database.Node{Address: host, Port: &port})
	if err == nil || result.IsAlive {
		t.Fatalf("health check against a foreign certificate succeeded: %+v", result)
	}
	if !strings.Contains(result.StatusMsg, "unknown authority") || !strings.Contains(result.StatusMsg, "not a node of this panel") {
		t.Fatalf("StatusMsg = %q", result.StatusMsg)
	}
}

func TestNodeRequestsIdentifyGoWave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Better-One"); got != "true" {
			t.Errorf("X-Better-One = %q, want true", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client()}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			var body interface{}
			if method == http.MethodPost {
				body = map[string]string{"test": "request"}
			}
			response, err := client.doRequest(t.Context(), method, server.URL, body)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", response.StatusCode)
			}
		})
	}
}
