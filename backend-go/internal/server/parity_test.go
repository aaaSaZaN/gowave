package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"remnawave-go/config"
	"remnawave-go/internal/testdb"

	"github.com/go-chi/chi/v5"
)

type contractCommand struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func loadContractCommands(t *testing.T) []contractCommand {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "contracts", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Version  string            `json:"version"`
		Commands []contractCommand `json:"commands"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.Version != "3.4.15" {
		t.Fatalf("contract fixture is %s, expected pinned 3.4.15", file.Version)
	}
	return file.Commands
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	db := testdb.Open(t)
	cfg := &config.Config{AppSecret: strings.Repeat("s", 32), DBDriver: "sqlite", JWTLifetime: 12}
	return NewApp(cfg, db)
}

// normalizePattern turns chi "{id}" and contract ":id" parameters into "*".
func normalizePattern(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") || (strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")) {
			parts[i] = "*"
		}
	}
	return "/" + strings.Join(parts, "/")
}

// TestContractRouteCoverage verifies every command of the pinned contract has a
// registered route with the same method and path shape.
func TestContractRouteCoverage(t *testing.T) {
	app := newTestApp(t)
	registered := map[string]bool{}
	err := chi.Walk(app.Router.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+normalizePattern(route)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, command := range loadContractCommands(t) {
		if !registered[command.Method+" "+normalizePattern(command.Path)] {
			missing = append(missing, command.Method+" "+command.Path+" ("+command.Name+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("%d contract routes are not registered:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}
