package cli

import (
	"bytes"
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
		"APP_SECRET":         "s3cr$t-with-dollar_and_more_than_32_chars!",
		"APP_PORT":           "3000",
		"DB_DRIVER":          "sqlite",
		"DATABASE_URL":       "/opt/gowave/gowave.sqlite3",
		"FRONT_END_DOMAIN":   "*",
		"TELEGRAM_BOT_TOKEN": "123:abc def",
	}
	env := buildDockerEnv(old, "/opt/gowave-docker", defaultGoWaveImage, "pgpass", "5432")
	for _, key := range []string{"DB_DRIVER", "DATABASE_URL"} {
		if _, ok := env[key]; ok {
			t.Errorf("%s must be dropped; the compose file sets it", key)
		}
	}
	for key, value := range map[string]string{
		"APP_SECRET":         old["APP_SECRET"],
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
