package cli

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestDockerComposeTemplateMatchesRepository(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(root, dockerComposeTemplate) {
		t.Fatal("backend-go/internal/cli/docker-compose.yml differs from the repository docker-compose.yml; run: cp docker-compose.yml backend-go/internal/cli/docker-compose.yml")
	}
}

func TestParseToDockerArgs(t *testing.T) {
	opts, err := parseToDockerArgs([]string{"--dir", "/srv/gw", "--image", "gowave:local", "--from", "/opt/gowave", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	want := toDockerOptions{DockerDir: "/srv/gw", Image: "gowave:local", FromDir: "/opt/gowave", Yes: true}
	if opts != want {
		t.Errorf("got %+v, want %+v", opts, want)
	}
	if opts, _ := parseToDockerArgs(nil); opts.DockerDir != defaultDockerDir {
		t.Errorf("default dir = %q", opts.DockerDir)
	}
	for _, args := range [][]string{{"--dir"}, {"--dir", "--yes"}, {"--bogus"}} {
		if _, err := parseToDockerArgs(args); err == nil {
			t.Errorf("parseToDockerArgs(%v) should fail", args)
		}
	}
}

func TestParseToDockerArgsFromUpstream(t *testing.T) {
	opts, err := parseToDockerArgs([]string{"--from-upstream", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.FromUpstream || opts.UpstreamDSN != "" {
		t.Errorf("bare flag: got %+v, want FromUpstream with an empty DSN for auto-detection", opts)
	}

	opts, err = parseToDockerArgs([]string{"--from-upstream", "postgresql://u:p@127.0.0.1:6767/db", "--dir", "/srv/gw"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.FromUpstream || opts.UpstreamDSN != "postgresql://u:p@127.0.0.1:6767/db" || opts.DockerDir != "/srv/gw" {
		t.Errorf("got %+v", opts)
	}
}

func TestComposeHostDSNReachesThePublishedPort(t *testing.T) {
	plan := &toDockerPlan{Env: map[string]string{
		"POSTGRES_USER":     "gowave",
		"POSTGRES_PASSWORD": "p@ss word/#",
		"POSTGRES_DB":       "gowave",
		"POSTGRES_PORT":     "5433",
	}}
	got := composeHostDSN(plan)
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("DSN %q does not parse: %v", got, err)
	}
	if parsed.Host != "127.0.0.1:5433" || parsed.Path != "/gowave" {
		t.Errorf("host/path = %q%q, want 127.0.0.1:5433/gowave", parsed.Host, parsed.Path)
	}
	if password, _ := parsed.User.Password(); password != "p@ss word/#" {
		t.Errorf("password = %q, want it escaped and preserved", password)
	}
	if parsed.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q, want disable", parsed.Query().Get("sslmode"))
	}
}

func TestResolveSourceSQLite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gowave.sqlite3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveSourceSQLite(dir, map[string]string{"DB_DRIVER": "sqlite", "DATABASE_URL": "gowave.sqlite3"})
	if err != nil || got != filepath.Join(dir, "gowave.sqlite3") {
		t.Errorf("relative path: got %q, %v", got, err)
	}
	if _, err := resolveSourceSQLite(dir, map[string]string{}); err == nil {
		t.Error("missing default gowave.db must be reported")
	}
	if _, err := resolveSourceSQLite(dir, map[string]string{"DATABASE_URL": "postgresql://u:p@db/gowave"}); err == nil {
		t.Error("a PostgreSQL install must be rejected")
	}
}

func TestBuildDockerEnvKeepsSecretsAndRendersLiterally(t *testing.T) {
	old := map[string]string{
		"PANEL_BASE_PATH":    "/private/panel/",
		"APP_SECRET":         "s3cr$t-with-dollar_and_more_than_32_chars!",
		"APP_PORT":           "3000",
		"DB_DRIVER":          "sqlite",
		"DATABASE_URL":       "/opt/gowave/gowave.sqlite3",
		"FRONT_END_DOMAIN":   "*",
		"TELEGRAM_BOT_TOKEN": "123:abc def",
		"GOWAVE_IMAGE":       "ghcr.io/aaasazan/gowave:dev",
	}
	env := buildDockerEnv(old, "/opt/gowave-docker", "pgpass", "5432")
	for _, key := range []string{"DB_DRIVER", "DATABASE_URL", "GOWAVE_IMAGE"} {
		if _, ok := env[key]; ok {
			t.Errorf("%s must be dropped; the compose file sets it", key)
		}
	}
	for key, value := range map[string]string{
		"APP_SECRET":         old["APP_SECRET"],
		"PANEL_BASE_PATH":    old["PANEL_BASE_PATH"],
		"TELEGRAM_BOT_TOKEN": old["TELEGRAM_BOT_TOKEN"],
		"GOWAVE_COMPOSE_DIR": "/opt/gowave-docker",
		"POSTGRES_PASSWORD":  "pgpass",
	} {
		if env[key] != value {
			t.Errorf("%s = %q, want %q", key, env[key], value)
		}
	}

	text, err := renderDockerEnv(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(text), []byte("APP_SECRET='s3cr$t-with-dollar_and_more_than_32_chars!'\n")) {
		t.Errorf("APP_SECRET must be single-quoted so compose does not interpolate $:\n%s", text)
	}
	parsed, err := godotenv.Unmarshal(text)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range env {
		if parsed[key] != value {
			t.Errorf("round trip %s = %q, want %q", key, parsed[key], value)
		}
	}

	if _, err := renderDockerEnv(map[string]string{"BAD": "it's"}); err == nil {
		t.Error("values with single quotes must be rejected")
	}
}

func TestRenderDockerComposePinsImage(t *testing.T) {
	compose, err := renderDockerCompose("")
	if err != nil || !bytes.Equal(compose, dockerComposeTemplate) {
		t.Fatalf("default image: %v", err)
	}

	for _, image := range []string{"ghcr.io/aaasazan/gowave:dev", "registry.local/gowave:custom"} {
		compose, err := renderDockerCompose(image)
		if err != nil {
			t.Fatalf("%s: %v", image, err)
		}
		if !bytes.Contains(compose, []byte("    image: "+image+"\n")) {
			t.Errorf("%s is not pinned in the compose file:\n%s", image, compose)
		}
		if bytes.Count(compose, []byte("    image: ")) != bytes.Count(dockerComposeTemplate, []byte("    image: ")) {
			t.Errorf("%s changed the number of image lines", image)
		}
	}

	for _, image := range []string{"two words", "line\nbreak", "image#comment"} {
		if _, err := renderDockerCompose(image); err == nil {
			t.Errorf("image %q must be rejected", image)
		}
	}
}
