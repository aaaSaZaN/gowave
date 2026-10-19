package cli

import (
	"bufio"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dockerComposeTemplate is the repository's docker-compose.yml; a test keeps
// the two copies identical.
//
//go:embed docker-compose.yml
var dockerComposeTemplate []byte

const (
	defaultDockerDir   = "/opt/gowave-docker"
	defaultGoWaveImage = "ghcr.io/aaasazan/gowave:latest"
	cliWrapperPath     = "/usr/local/bin/gowave"
	systemdUnit        = "gowave.service"
	// Fixed by the `name:` and `container_name:` entries of the compose file.
	dockerVolumeName = "gowave_postgres_data"
)

var dockerContainerNames = []string{"gowave", "gowave-postgres"}

// envKeysReplacedInDocker are host-install settings the compose file sets itself.
var envKeysReplacedInDocker = map[string]bool{"DB_DRIVER": true, "DATABASE_URL": true, "APP_HOST": true}

type toDockerOptions struct {
	FromDir   string
	DockerDir string
	Image     string
	Yes       bool
}

// toDockerPlan is everything derived from the existing install before any
// change is made.
type toDockerPlan struct {
	toDockerOptions
	SourceDB    string
	AppPort     string
	Env         map[string]string
	HasSystemd  bool
	WasActive   bool
	LocalImage  bool
	WrapperFrom string
}

func parseToDockerArgs(args []string) (toDockerOptions, error) {
	opts := toDockerOptions{DockerDir: defaultDockerDir}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() (string, error) {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", fmt.Errorf("%s needs a value", arg)
			}
			i++
			return args[i], nil
		}
		var err error
		switch arg {
		case "--yes", "-y":
			opts.Yes = true
		case "--from":
			opts.FromDir, err = value()
		case "--dir":
			opts.DockerDir, err = value()
		case "--image":
			opts.Image, err = value()
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
		if err != nil {
			return opts, err
		}
	}
	return opts, nil
}

// RunMigrateToDocker moves a systemd/SQLite install into Docker with PostgreSQL.
func RunMigrateToDocker(args []string, reader *bufio.Reader) error {
	opts, err := parseToDockerArgs(args)
	if err != nil {
		return err
	}
	plan, err := prepareToDocker(opts)
	if err != nil {
		return err
	}

	printToDockerPlan(plan)
	if !plan.Yes {
		fmt.Print("\nContinue? Type 'yes' to migrate: ")
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(answer) != "yes" {
			return errors.New("cancelled")
		}
	}
	return executeToDocker(plan)
}

func prepareToDocker(opts toDockerOptions) (*toDockerPlan, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, errors.New("docker is not installed (https://docs.docker.com/engine/install/)")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return nil, errors.New("the docker compose plugin is not available")
	}

	if opts.FromDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("locate current install: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		opts.FromDir = filepath.Dir(exe)
	}
	fromDir, err := filepath.Abs(opts.FromDir)
	if err != nil {
		return nil, err
	}
	opts.FromDir = fromDir
	if opts.DockerDir, err = filepath.Abs(opts.DockerDir); err != nil {
		return nil, err
	}

	envPath := filepath.Join(opts.FromDir, ".env")
	oldEnv, err := godotenv.Read(envPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w (pass --from <install dir>)", envPath, err)
	}
	if len(oldEnv["APP_SECRET"]) < 32 {
		return nil, fmt.Errorf("%s has no valid APP_SECRET; it must be carried over to keep admin logins working", envPath)
	}

	sourceDB, err := resolveSourceSQLite(opts.FromDir, oldEnv)
	if err != nil {
		return nil, err
	}
	if err := checkSQLiteFileForeignKeys(sourceDB); err != nil {
		return nil, err
	}

	if _, err := os.Stat(filepath.Join(opts.DockerDir, "docker-compose.yml")); err == nil {
		return nil, fmt.Errorf("%s already contains docker-compose.yml; choose another --dir", opts.DockerDir)
	}
	for _, name := range dockerContainerNames {
		if exec.Command("docker", "container", "inspect", name).Run() == nil {
			return nil, fmt.Errorf("a Docker container named %q already exists; remove it first", name)
		}
	}
	if exec.Command("docker", "volume", "inspect", dockerVolumeName).Run() == nil {
		return nil, fmt.Errorf("Docker volume %s already exists (an earlier GoWave PostgreSQL); remove it with `docker volume rm %s` if it is not needed", dockerVolumeName, dockerVolumeName)
	}

	appPort := oldEnv["APP_PORT"]
	if appPort == "" {
		appPort = "3000"
	}
	password, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	pgPort := "5432"
	if !portFree("127.0.0.1:" + pgPort) {
		if pgPort, err = freeLoopbackPort(); err != nil {
			return nil, err
		}
	}
	image := opts.Image
	if image == "" {
		image = defaultGoWaveImage
	}

	plan := &toDockerPlan{
		toDockerOptions: opts,
		SourceDB:        sourceDB,
		AppPort:         appPort,
		Env:             buildDockerEnv(oldEnv, opts.DockerDir, image, password, pgPort),
		HasSystemd:      exec.Command("systemctl", "cat", systemdUnit).Run() == nil,
	}
	plan.WasActive = plan.HasSystemd && exec.Command("systemctl", "is-active", "--quiet", systemdUnit).Run() == nil
	plan.LocalImage = opts.Image != "" && exec.Command("docker", "image", "inspect", opts.Image).Run() == nil
	if info, err := os.Lstat(cliWrapperPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		plan.WrapperFrom, _ = os.Readlink(cliWrapperPath)
	}
	if !plan.HasSystemd && !portFree(":"+appPort) {
		return nil, fmt.Errorf("port %s is in use and %s was not found; stop the running panel first", appPort, systemdUnit)
	}
	return plan, nil
}

// checkSQLiteFileForeignKeys runs the copy's foreign-key check read-only,
// before the panel is stopped.
func checkSQLiteFileForeignKeys(path string) error {
	db, err := gorm.Open(sqlite.Open(sqliteReadOnlyURI(path)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer closeGormDB(db)
	return checkSQLiteForeignKeys(db)
}

func resolveSourceSQLite(fromDir string, env map[string]string) (string, error) {
	driver := strings.ToLower(env["DB_DRIVER"])
	url := env["DATABASE_URL"]
	if driver == "postgres" || strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		return "", errors.New("this install already uses PostgreSQL; only SQLite installs can be moved with to-docker")
	}
	if url == "" {
		url = "gowave.db"
	}
	if !filepath.IsAbs(url) {
		// The systemd unit starts the panel from its install directory.
		url = filepath.Join(fromDir, url)
	}
	if info, err := os.Stat(url); err != nil || info.IsDir() {
		return "", fmt.Errorf("SQLite database %s not found", url)
	}
	return url, nil
}

// buildDockerEnv keeps every setting of the host install (APP_SECRET hashes
// admin passwords and API tokens, so it must not change) and adds the Docker
// specific ones.
func buildDockerEnv(oldEnv map[string]string, dockerDir, image, pgPassword, pgPort string) map[string]string {
	env := make(map[string]string, len(oldEnv)+6)
	for key, value := range oldEnv {
		if !envKeysReplacedInDocker[key] {
			env[key] = value
		}
	}
	env["GOWAVE_IMAGE"] = image
	env["GOWAVE_COMPOSE_DIR"] = dockerDir
	env["POSTGRES_USER"] = "gowave"
	env["POSTGRES_PASSWORD"] = pgPassword
	env["POSTGRES_DB"] = "gowave"
	env["POSTGRES_PORT"] = pgPort
	return env
}

var plainEnvValue = regexp.MustCompile(`^[A-Za-z0-9_./:@*,+=-]*$`)

// renderDockerEnv writes values so that docker compose reads them literally:
// plain values unquoted, anything else single-quoted (no $ interpolation).
func renderDockerEnv(env map[string]string) (string, error) {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	out.WriteString("# Generated by `gowave migrate to-docker`. Every key is passed to the panel container.\n")
	for _, key := range keys {
		value := env[key]
		switch {
		case plainEnvValue.MatchString(value):
			fmt.Fprintf(&out, "%s=%s\n", key, value)
		case !strings.ContainsAny(value, "'\n"):
			fmt.Fprintf(&out, "%s='%s'\n", key, value)
		default:
			return "", fmt.Errorf("value of %s contains a single quote or newline; edit the old .env and retry", key)
		}
	}
	return out.String(), nil
}

func printToDockerPlan(plan *toDockerPlan) {
	fmt.Println("\n========================================================")
	fmt.Println("  Move GoWave to Docker (PostgreSQL)")
	fmt.Println("========================================================")
	fmt.Printf("Current install:  %s\n", plan.FromDir)
	fmt.Printf("SQLite database:  %s (read only, kept for rollback)\n", plan.SourceDB)
	fmt.Printf("New install:      %s (docker-compose.yml + .env)\n", plan.DockerDir)
	fmt.Printf("Image:            %s\n", plan.Env["GOWAVE_IMAGE"])
	fmt.Printf("Panel port:       %s (unchanged)\n", plan.AppPort)
	fmt.Println("\nSteps:")
	fmt.Println("  1. Write the compose project, reusing APP_SECRET and all settings from the current .env")
	fmt.Println("  2. Pull images and start PostgreSQL")
	if plan.HasSystemd {
		fmt.Printf("  3. Stop %s (the panel is offline from here until step 5)\n", systemdUnit)
	} else {
		fmt.Println("  3. (no systemd unit found; the old panel must already be stopped)")
	}
	fmt.Println("  4. Copy all data from SQLite into PostgreSQL")
	fmt.Println("  5. Start the panel in Docker and wait until it is healthy")
	if plan.HasSystemd {
		fmt.Printf("  6. Disable %s and point `gowave` at the container\n", systemdUnit)
	}
	fmt.Println("\nIf any step fails, the Docker project is removed and the old panel is started again.")
}

func executeToDocker(plan *toDockerPlan) error {
	envText, err := renderDockerEnv(plan.Env)
	if err != nil {
		return err
	}
	compose := func(args ...string) error {
		return runDockerCompose(plan.DockerDir, plan.Env, args...)
	}

	step := func(n int, text string) { fmt.Printf("\n[%d/6] %s\n", n, text) }

	step(1, "Writing "+plan.DockerDir)
	if err := os.MkdirAll(plan.DockerDir, 0o755); err != nil {
		return err
	}
	composePath := filepath.Join(plan.DockerDir, "docker-compose.yml")
	envPath := filepath.Join(plan.DockerDir, ".env")
	if err := os.WriteFile(composePath, dockerComposeTemplate, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(envPath, []byte(envText), 0o600); err != nil {
		_ = os.Remove(composePath)
		return err
	}
	removeProject := func() {
		_ = compose("down", "--volumes", "--remove-orphans")
		_ = os.Remove(composePath)
		_ = os.Remove(envPath)
	}

	step(2, "Pulling images and starting PostgreSQL")
	pull := []string{"pull"}
	if plan.LocalImage {
		pull = append(pull, "postgres")
	}
	if err := compose(pull...); err != nil {
		removeProject()
		return fmt.Errorf("pull images: %w", err)
	}
	if err := compose("up", "-d", "--wait", "postgres"); err != nil {
		removeProject()
		return fmt.Errorf("start PostgreSQL: %w", err)
	}

	stopped := false
	rollback := func(cause error) error {
		fmt.Println("\n[!] Rolling back...")
		removeProject()
		if stopped && plan.WasActive {
			if err := exec.Command("systemctl", "start", systemdUnit).Run(); err != nil {
				return fmt.Errorf("%w; rollback could not restart %s: %v", cause, systemdUnit, err)
			}
			fmt.Printf("[*] %s is running again on SQLite\n", systemdUnit)
		}
		return cause
	}

	step(3, "Stopping the current panel")
	if plan.HasSystemd {
		if err := runVerbose("systemctl", "stop", systemdUnit); err != nil {
			return rollback(fmt.Errorf("stop %s: %w", systemdUnit, err))
		}
		stopped = true
	}
	if !waitPortFree(":"+plan.AppPort, 15*time.Second) {
		return rollback(fmt.Errorf("port %s is still in use after stopping the panel", plan.AppPort))
	}

	step(4, "Copying data into PostgreSQL")
	srcDir := filepath.Dir(plan.SourceDB)
	if err := compose("run", "--rm", "--no-deps", "-v", srcDir+":/src",
		"gowave", "migrate", "sqlite-to-pg", "/src/"+filepath.Base(plan.SourceDB)); err != nil {
		return rollback(fmt.Errorf("copy data: %w", err))
	}

	step(5, "Starting the panel in Docker")
	if err := compose("up", "-d", "--wait", "--wait-timeout", "180"); err != nil {
		return rollback(fmt.Errorf("start panel: %w", err))
	}
	if err := checkPanelHealth(plan.AppPort); err != nil {
		return rollback(err)
	}

	step(6, "Finishing")
	if plan.HasSystemd {
		if err := runVerbose("systemctl", "disable", systemdUnit); err != nil {
			fmt.Printf("[!] Could not disable %s: %v. Disable it manually so it does not start on boot.\n", systemdUnit, err)
		}
		installCLIWrapper(plan)
	}

	fmt.Println("\n[✔] GoWave now runs in Docker with PostgreSQL.")
	fmt.Printf("    Project:  %s\n", plan.DockerDir)
	fmt.Printf("    Logs:     cd %s && docker compose logs -f gowave\n", plan.DockerDir)
	fmt.Printf("    Update:   cd %s && docker compose pull && docker compose up -d\n", plan.DockerDir)
	if plan.HasSystemd {
		fmt.Printf("    CLI:      gowave <command>   (runs inside the container)\n")
	}
	fmt.Printf("\nThe SQLite database %s was not modified. To go back:\n", plan.SourceDB)
	fmt.Printf("    cd %s && docker compose down\n", plan.DockerDir)
	if plan.HasSystemd && plan.WrapperFrom != "" {
		fmt.Printf("    ln -sf %s %s\n", plan.WrapperFrom, cliWrapperPath)
	}
	if plan.HasSystemd {
		fmt.Printf("    systemctl enable --now %s\n", systemdUnit)
	}
	fmt.Println("Changes made in the Docker panel are not copied back to SQLite.")
	return nil
}

// installCLIWrapper replaces the host binary's symlink so rescue commands
// reach the PostgreSQL panel instead of the abandoned SQLite file.
func installCLIWrapper(plan *toDockerPlan) {
	if info, err := os.Lstat(cliWrapperPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return
	}
	script := fmt.Sprintf(`#!/bin/sh
# Installed by "gowave migrate to-docker": GoWave runs in Docker (%[1]s).
exec docker compose --project-directory %[1]s exec gowave /app/gowave "$@"
`, plan.DockerDir)
	_ = os.Remove(cliWrapperPath)
	if err := os.WriteFile(cliWrapperPath, []byte(script), 0o755); err != nil {
		fmt.Printf("[!] Could not install %s: %v\n", cliWrapperPath, err)
	}
}

func runDockerCompose(dir string, env map[string]string, args ...string) error {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// The old .env was loaded into this process; drop those keys so compose
	// reads the generated .env instead of stale shell variables.
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, generated := env[key]; !generated && !envKeysReplacedInDocker[key] {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	return cmd.Run()
}

func runVerbose(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func checkPanelHealth(port string) error {
	client := http.Client{Timeout: 5 * time.Second}
	url := "http://127.0.0.1:" + port + "/api/system/health"
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		if time.Now().After(deadline) {
			if err == nil {
				err = fmt.Errorf("status %d", resp.StatusCode)
			}
			return fmt.Errorf("panel did not become healthy at %s: %v", url, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func portFree(addr string) bool {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func waitPortFree(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for !portFree(addr) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
	return true
}

func freeLoopbackPort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	return port, err
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
