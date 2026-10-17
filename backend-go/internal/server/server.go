package server

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gowave/config"
	"gowave/internal/auth"
	"gowave/internal/cli"
	"gowave/internal/configprofiles"
	"gowave/internal/cron"
	"gowave/internal/database"
	"gowave/internal/infrabilling"
	"gowave/internal/jobs"
	"gowave/internal/keygen"
	"gowave/internal/middleware"
	"gowave/internal/nodes"
	"gowave/internal/nodessh"
	"gowave/internal/notify"
	"gowave/internal/prometheus"
	"gowave/internal/squads"
	"gowave/internal/stubs"
	"gowave/internal/subscription"
	"gowave/internal/subtemplates"
	"gowave/internal/system"
	"gowave/internal/tokens"
	"gowave/internal/users"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"
)

func ensureGlobalSymlink() {
	if os.Geteuid() != 0 {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	symlinkPath := "/usr/local/bin/gowave"
	if info, err := os.Lstat(symlinkPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
		// A regular file is the wrapper `migrate to-docker` installs to forward
		// CLI commands into the container; a host binary must not replace it.
		return
	}
	target, err := os.Readlink(symlinkPath)
	if err == nil && target == exe {
		return
	}
	_ = os.Remove(symlinkPath)
	_ = os.Symlink(exe, symlinkPath)
}

func Run() {
	// The panel keeps every timestamp in UTC like upstream's container; pgx and
	// encoding/json would otherwise render the host's zone.
	time.Local = time.UTC
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Println(system.AppVersion)
		return
	}
	cfg := config.Load()
	if cli.IsPostgresToSQLiteMigrationCommand(args) {
		cli.RunRescue(nil, cfg, args)
		return
	}

	ensureGlobalSymlink()

	db, err := database.Init(cfg.DBDriver, cfg.DatabaseURL)
	if err != nil {
		fmt.Printf("Database init failed: %v\n", err)
		os.Exit(1)
	}
	if cfg.DBMaxConnections > 0 {
		database.SetMaxConnections(db, cfg.DBMaxConnections)
	}

	if err := database.AutoMigrate(db); err != nil {
		fmt.Printf("Auto migration failed: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		firstArg := os.Args[1]
		if firstArg == "--rescue" || firstArg == "rescue" || firstArg == "cli" || firstArg == "--cli" {
			cli.RunRescue(db, cfg, os.Args[2:])
			return
		}
		if firstArg == "import" || firstArg == "--import" || firstArg == "--import-pasarguard" || firstArg == "import-pasarguard" || firstArg == "migrate" || firstArg == "--migrate" || firstArg == "--migrate-pg-to-sqlite" || firstArg == "--migrate-sqlite-to-pg" || firstArg == "migrate-sqlite-to-pg" {
			cli.RunRescue(db, cfg, os.Args[1:])
			return
		}
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := NewApp(cfg, db)
	app.Start(ctx)

	serverAddr := net.JoinHostPort(cfg.AppHost, cfg.AppPort)
	if !cfg.IsLoopbackHost() {
		fmt.Printf("WARNING: APP_HOST=%s exposes the panel without HTTPS. Keep APP_HOST=127.0.0.1 and serve the panel through a reverse proxy (Caddy/Nginx).\n", cfg.AppHost)
	}
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           app.Router,
		ReadHeaderTimeout: 15 * time.Second,
		// Backup uploads and large exports outlive the header timeout; the SSH
		// WebSocket is hijacked, which clears both deadlines.
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("GoWave running on %s\n", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Listen failed: %v\n", err)
		}
	}()

	<-quit
	fmt.Println("Shutting down GoWave server...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server forced shutdown: %v\n", err)
	}
	// Background writers flush what requests queued before the shutdown.
	app.wait(5 * time.Second)
	fmt.Println("Server exiting")
}

// App holds the HTTP router and the background workers. Construction has no
// side effects so tests can inspect routes; Start launches the workers.
type App struct {
	Router http.Handler
	start  func(ctx context.Context)
	wait   func(timeout time.Duration)
}

// Start launches the scheduler, job workers and node health checks; they stop
// when ctx is cancelled.
func (a *App) Start(ctx context.Context) { a.start(ctx) }

// NewApp wires services and registers every route once.
func NewApp(cfg *config.Config, db *gorm.DB) *App {
	scheduler := cron.New(db)
	scheduler.SetConfig(cfg)

	authHandler := auth.NewHandler(db, cfg.AppSecret, cfg.JWTLifetime)
	users.SetShortUUIDLength(cfg.ShortUUIDLength)
	users.SetSubPublicDomain(cfg.SubPublicDomain)
	notifier, err := notify.New(cfg)
	if err != nil {
		// Like upstream, an invalid notifications config stops startup.
		fmt.Fprintf(os.Stderr, "Invalid notifications config: %v\n", err)
		os.Exit(1)
	}
	notify.SetDefault(notifier)
	userService := users.NewService(db)
	userHandler := users.NewHandler(userService, cfg.SubPublicDomain)
	keygenSvc := keygen.NewService(db)
	var nodeClient *nodes.Client
	if km, err := keygenSvc.GetMasterKeygen(); err == nil {
		nodeClient, _ = nodes.NewNodeClient([]byte(km.CACert), []byte(km.ClientCert), []byte(km.ClientKey), []byte(km.PrivKey), []byte(km.PubKey))
	}
	nodeService := nodes.NewService(db, nodeClient)
	nodeService.SetUsageOptions(nodes.UsageOptions{IgnoreBelowBytes: cfg.UserUsageIgnoreBelowBytes, DisableUserRecords: cfg.Service.DisableUserUsageRecords})
	workQueue := jobs.New(db)
	notifier.Register(workQueue)
	workQueue.Register("user-sync", nodeService.ApplyUserJob)
	workQueue.Register("node-start", nodeService.ApplyNodeJob)
	workQueue.Register("node-stop", nodeService.ApplyNodeJob)
	workQueue.Register("node-retire", nodeService.ApplyRetireJob)
	workQueue.Register("torrent-collect", nodeService.ApplyCollectJob)
	for _, kind := range []string{"plugin-sync", "plugin-exec", "connections-drop", "geocheck"} {
		workQueue.Register(kind, nodeService.ApplyCommandJob)
	}
	userService.SetSyncer(nodeService)
	scheduler.SetSyncer(nodeService)
	scheduler.SetNodeFormatter(nodeService.EventData)
	nodeHandler := nodes.NewHandler(nodeService)
	cpService := configprofiles.NewService(db, nodeService)
	cpService.SetSyncer(nodeService)
	cpHandler := configprofiles.NewHandler(cpService)
	subHandler := subscription.NewHandler(db, cfg.SubPublicDomain)
	metricsExporter := prometheus.NewExporter(db, nodeService)
	subHandler.SetDisableHistory(cfg.Service.DisableSrhRecords)
	sysHandler := system.NewHandler(db)

	squadsHandler := squads.NewHandler(db)
	subtemplatesHandler := subtemplates.NewHandler(db)
	tokensHandler := tokens.NewHandler(db, cfg.AppSecret)
	infrabillingHandler := infrabilling.NewHandler(db)
	sshHandler := nodessh.NewHandler(db, cfg.AppSecret)
	stubHandler := stubs.NewHandler(db, cfg.AppSecret, cfg.JWTLifetime, nodeClient)
	stubHandler.SetSubPublicDomain(cfg.SubPublicDomain)
	stubHandler.SetConfig(cfg)
	stubHandler.SetClientSource(nodeService.Client)
	sysHandler.OnRestored = func() {
		nodeService.ResetClient()
		go nodeService.SyncAllUsersToConnectedNodes()
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.StripSlashes)
	r.Use(middleware.Cors())
	r.Use(middleware.Logger)
	r.Use(system.RouteCounterMiddleware)
	r.Use(middleware.ContractStatus)
	system.NewTools(db, cfg.AppSecret).Register(r)
	r.Get("/api/node-ssh/ws", sshHandler.Socket)

	r.Get("/api/system/health", sysHandler.GetHealth)
	r.Head("/api/system/health", sysHandler.GetHealth)
	r.Get("/api/auth/status", authHandler.GetStatus)
	r.Head("/api/auth/status", authHandler.GetStatus)
	r.Post("/api/auth/login", authHandler.Login)
	r.Post("/api/auth/register", authHandler.Register)

	r.Get("/api/auth/passkey/authentication/options", stubHandler.GetPasskeyAuthOptions)
	r.Post("/api/auth/passkey/authentication/verify", stubHandler.VerifyPasskeyAuth)
	r.Post("/api/auth/oauth2/authorize", stubHandler.OAuth2Authorize)
	r.Post("/api/auth/oauth2/callback", stubHandler.OAuth2Callback)
	r.Post("/api/auth/oauth2/tg/callback", stubHandler.OAuth2TelegramCallback)

	r.Get("/api/sub/{shortUuid}", subHandler.GetSubscription)
	r.Get("/api/sub/{shortUuid}/info", subHandler.GetSubscriptionInfo)
	r.Get("/api/sub/{shortUuid}/{clientType}", subHandler.GetSubscription)

	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(cfg.AppSecret, db))

		protected.Get("/api/system/metadata", sysHandler.GetMetadata)
		protected.Get("/api/system/updates/check", sysHandler.CheckUpdate)
		protected.Post("/api/system/updates/apply", sysHandler.ApplyUpdate)
		protected.Get("/api/tokens", tokensHandler.GetApiTokens)
		protected.Post("/api/tokens", tokensHandler.CreateApiToken)
		protected.Delete("/api/tokens/{uuid}", tokensHandler.DeleteApiToken)
		protected.Get("/api/tokens/scopes", tokensHandler.GetScopes)
		protected.Post("/api/tokens/ott", tokensHandler.GetOtt)

		protected.Get("/api/subscription-settings", sysHandler.GetSubscriptionSettings)
		protected.Patch("/api/subscription-settings", sysHandler.UpdateSubscriptionSettings)
		protected.Get("/api/system/stats", sysHandler.GetStats)
		protected.Get("/api/system/stats/bandwidth", sysHandler.GetBandwidthStats)
		protected.Get("/api/system/stats/nodes", sysHandler.GetNodesStatistics)
		protected.Get("/api/system/nodes/metrics", sysHandler.GetNodesMetrics)
		protected.Get("/api/system/stats/recap", sysHandler.GetRecap)
		protected.Get("/api/system/stats/http", sysHandler.GetHttpStats)
		protected.Get("/api/remnawave-settings", sysHandler.GetSettings)
		protected.Patch("/api/remnawave-settings", sysHandler.UpdateSettings)
		protected.Post("/api/system/actions/restore", sysHandler.RestoreBackup)

		protected.Get("/api/users", userHandler.GetUsers)
		protected.Get("/api/users/stream", userHandler.GetUsersStream)
		protected.Get("/api/users/tags", userHandler.GetTags)
		protected.Post("/api/users", userHandler.CreateUser)
		protected.Patch("/api/users", userHandler.PatchUser)
		protected.Post("/api/users/resolve", userHandler.ResolveUser)
		protected.Post("/api/users/actions/resolve", userHandler.ResolveUser)

		protected.Post("/api/users/bulk/all/extend-expiration-date", userHandler.BulkAllExtendExpirationDate)
		protected.Post("/api/users/bulk/all/reset-traffic", userHandler.BulkAllResetTraffic)
		protected.Post("/api/users/bulk/all/update", userHandler.BulkAllUpdate)
		protected.Post("/api/users/bulk/delete", userHandler.BulkDelete)
		protected.Post("/api/users/bulk/delete-by-status", userHandler.BulkDeleteByStatus)
		protected.Post("/api/users/bulk/extend-expiration-date", userHandler.BulkExtendExpirationDate)
		protected.Post("/api/users/bulk/reset-traffic", userHandler.BulkResetTraffic)
		protected.Post("/api/users/bulk/revoke-subscription", userHandler.BulkRevokeSubscription)
		protected.Post("/api/users/bulk/update", userHandler.BulkUpdate)
		protected.Post("/api/users/bulk/update-squads", userHandler.BulkUpdateSquads)

		protected.Post("/api/users/bulk-actions", userHandler.BulkUserAction)
		protected.Post("/api/users/bulk-actions/reset-traffic", userHandler.BulkUserAction)
		protected.Post("/api/users/bulk-actions/delete", userHandler.BulkUserAction)
		protected.Post("/api/users/bulk-actions/status-modification", userHandler.BulkUserAction)
		protected.Post("/api/users/bulk-actions/profile-modification", userHandler.BulkUserAction)
		protected.Post("/api/users/bulk-actions/template-modification", userHandler.BulkUserAction)

		protected.Get("/api/users/{userId}", userHandler.GetUser)
		protected.Patch("/api/users/{userId}", userHandler.UpdateUser)
		protected.Delete("/api/users/{userId}", userHandler.DeleteUser)
		protected.Get("/api/users/by-short-uuid/{shortUuid}", userHandler.GetUserByShortUuid)
		protected.Get("/api/users/by-username/{username}", userHandler.GetUserByUsername)
		protected.Get("/api/users/{userId}/accessible-nodes", userHandler.GetUserAccessibleNodes)
		protected.Get("/api/users/{userId}/subscription-request-history", userHandler.GetUserSubscriptionRequestHistory)
		protected.Post("/api/users/{userId}/actions/reset-traffic", userHandler.ResetTraffic)
		protected.Post("/api/users/{userId}/actions/enable", userHandler.EnableUser)
		protected.Post("/api/users/{userId}/actions/disable", userHandler.DisableUser)
		protected.Post("/api/users/{userId}/actions/extend", userHandler.ExtendUser)
		protected.Post("/api/users/{userId}/actions/revoke", userHandler.RevokeUser)

		protected.Get("/api/nodes", nodeHandler.GetNodes)
		protected.Get("/api/nodes/tags", nodeHandler.GetTags)
		protected.Post("/api/nodes", nodeHandler.CreateNode)
		protected.Patch("/api/nodes", nodeHandler.PatchNode)
		protected.Get("/api/nodes/{uuid}", nodeHandler.GetNode)
		protected.Get("/api/nodes/{uuid}/logs", nodeHandler.GetNodeLogs)
		protected.Get("/api/nodes/{uuid}/updates", nodeHandler.GetNodeUpdates)
		protected.Post("/api/nodes/{uuid}/updates/apply", nodeHandler.ApplyNodeUpdate)
		protected.Patch("/api/nodes/{uuid}", nodeHandler.UpdateNode)
		protected.Delete("/api/nodes/{uuid}", nodeHandler.DeleteNode)
		protected.Post("/api/nodes/{uuid}/actions/enable", nodeHandler.EnableNode)
		protected.Post("/api/nodes/{uuid}/actions/disable", nodeHandler.DisableNode)
		protected.Post("/api/nodes/{uuid}/actions/reset-traffic", nodeHandler.ResetTraffic)
		protected.Post("/api/nodes/{uuid}/actions/restart", nodeHandler.RestartNode)
		protected.Post("/api/nodes/actions/restart-all", nodeHandler.RestartAllNodes)
		protected.Post("/api/nodes/actions/reorder", nodeHandler.ReorderNodes)
		protected.Post("/api/nodes/bulk-actions", nodeHandler.BulkActions)
		protected.Post("/api/nodes/bulk-actions/profile-modification", nodeHandler.BulkActions)
		protected.Post("/api/nodes/bulk-actions/update", nodeHandler.BulkActions)

		protected.Get("/api/hosts/tags", cpHandler.GetHostsTags)
		protected.Get("/api/hosts", cpHandler.GetHosts)
		protected.Post("/api/hosts", cpHandler.CreateHost)
		protected.Get("/api/hosts/{uuid}", cpHandler.GetHost)
		protected.Patch("/api/hosts", cpHandler.UpdateHost)
		protected.Delete("/api/hosts/{uuid}", cpHandler.DeleteHost)
		protected.Post("/api/hosts/preview", cpHandler.Preview)
		protected.Post("/api/hosts/actions/clone", cpHandler.CloneHost)
		protected.Post("/api/hosts/actions/reorder", cpHandler.ReorderHosts)
		protected.Post("/api/hosts/bulk/delete", cpHandler.BulkDeleteHosts)
		protected.Post("/api/hosts/bulk/disable", cpHandler.BulkDisableHosts)
		protected.Post("/api/hosts/bulk/enable", cpHandler.BulkEnableHosts)
		protected.Patch("/api/hosts/bulk/update", cpHandler.BulkUpdateHosts)

		protected.Get("/api/config-profiles/tags", cpHandler.GetConfigProfilesTags)
		protected.Patch("/api/config-profiles/tags", cpHandler.SetConfigProfilesTags)
		protected.Get("/api/config-profiles/inbounds", cpHandler.GetAllInbounds)
		protected.Get("/api/config-profiles", cpHandler.GetProfiles)
		protected.Post("/api/config-profiles", cpHandler.CreateConfigProfile)
		protected.Get("/api/config-profiles/{uuid}", cpHandler.GetConfigProfile)
		protected.Patch("/api/config-profiles", cpHandler.UpdateConfigProfile)
		protected.Patch("/api/config-profiles/{uuid}", cpHandler.UpdateConfigProfile)
		protected.Delete("/api/config-profiles/{uuid}", cpHandler.DeleteConfigProfile)
		protected.Post("/api/config-profiles/actions/reorder", cpHandler.ReorderConfigProfiles)
		protected.Get("/api/config-profiles/{uuid}/inbounds", cpHandler.GetProfileInbounds)
		protected.Get("/api/config-profiles/{uuid}/computed-config", cpHandler.GetComputedConfig)

		protected.Get("/api/internal-squads", squadsHandler.GetInternalSquads)
		protected.Post("/api/internal-squads", squadsHandler.CreateInternalSquad)
		protected.Get("/api/internal-squads/{uuid}", squadsHandler.GetInternalSquad)
		protected.Patch("/api/internal-squads", squadsHandler.UpdateInternalSquad)
		protected.Delete("/api/internal-squads/{uuid}", squadsHandler.DeleteInternalSquad)
		protected.Post("/api/internal-squads/actions/reorder", squadsHandler.ReorderInternalSquads)
		protected.Get("/api/internal-squads/tags", squadsHandler.GetInternalSquadsTags)
		protected.Patch("/api/internal-squads/tags", squadsHandler.SetInternalSquadsTags)
		protected.Get("/api/internal-squads/{uuid}/accessible-nodes", squadsHandler.GetInternalSquadAccessibleNodes)
		protected.Post("/api/internal-squads/{uuid}/bulk-actions/add-users", squadsHandler.SquadBulkAction)
		protected.Post("/api/internal-squads/{uuid}/bulk-actions/add-many-users", squadsHandler.SquadBulkAction)
		protected.Delete("/api/internal-squads/{uuid}/bulk-actions/remove-users", squadsHandler.SquadBulkAction)
		protected.Delete("/api/internal-squads/{uuid}/bulk-actions/remove-many-users", squadsHandler.SquadBulkAction)

		protected.Get("/api/external-squads", squadsHandler.GetExternalSquads)
		protected.Post("/api/external-squads", squadsHandler.CreateExternalSquad)
		protected.Get("/api/external-squads/{uuid}", squadsHandler.GetExternalSquad)
		protected.Patch("/api/external-squads", squadsHandler.UpdateExternalSquad)
		protected.Delete("/api/external-squads/{uuid}", squadsHandler.DeleteExternalSquad)
		protected.Post("/api/external-squads/actions/reorder", squadsHandler.ReorderExternalSquads)
		protected.Get("/api/external-squads/tags", squadsHandler.GetExternalSquadsTags)
		protected.Patch("/api/external-squads/tags", squadsHandler.SetExternalSquadsTags)
		protected.Post("/api/external-squads/{uuid}/bulk-actions/add-users", squadsHandler.SquadBulkAction)
		protected.Delete("/api/external-squads/{uuid}/bulk-actions/remove-users", squadsHandler.SquadBulkAction)

		protected.Get("/api/subscription-templates", subtemplatesHandler.GetAllTemplates)
		protected.Post("/api/subscription-templates", subtemplatesHandler.CreateTemplate)
		protected.Get("/api/subscription-templates/{uuid}", subtemplatesHandler.GetTemplateByUuid)
		protected.Patch("/api/subscription-templates", subtemplatesHandler.UpdateTemplate)
		protected.Delete("/api/subscription-templates/{uuid}", subtemplatesHandler.DeleteTemplate)
		protected.Post("/api/subscription-templates/actions/reorder", subtemplatesHandler.ReorderTemplates)
		protected.Get("/api/subscription-templates/tags", subtemplatesHandler.GetTemplateTags)
		protected.Patch("/api/subscription-templates/tags", subtemplatesHandler.SetTemplateTags)

		protected.Get("/api/subscription-page-configs", subtemplatesHandler.GetAllConfigs)
		protected.Post("/api/subscription-page-configs", subtemplatesHandler.CreateConfig)
		protected.Get("/api/subscription-page-configs/{uuid}", subtemplatesHandler.GetConfigByUuid)
		protected.Patch("/api/subscription-page-configs", subtemplatesHandler.UpdateConfig)
		protected.Delete("/api/subscription-page-configs/{uuid}", subtemplatesHandler.DeleteConfig)
		protected.Post("/api/subscription-page-configs/actions/clone", subtemplatesHandler.CloneConfig)
		protected.Post("/api/subscription-page-configs/actions/reorder", subtemplatesHandler.ReorderConfigs)
		protected.Get("/api/subscription-page-configs/tags", subtemplatesHandler.GetConfigTags)
		protected.Patch("/api/subscription-page-configs/tags", subtemplatesHandler.SetConfigTags)

		protected.Get("/api/infra-billing/providers", infrabillingHandler.GetProviders)
		protected.Post("/api/infra-billing/providers", infrabillingHandler.CreateProvider)
		protected.Get("/api/infra-billing/providers/{uuid}", infrabillingHandler.GetProvider)
		protected.Patch("/api/infra-billing/providers", infrabillingHandler.UpdateProvider)
		protected.Delete("/api/infra-billing/providers/{uuid}", infrabillingHandler.DeleteProvider)
		protected.Get("/api/infra-billing/nodes", infrabillingHandler.GetNodes)
		protected.Post("/api/infra-billing/nodes", infrabillingHandler.CreateNode)
		protected.Patch("/api/infra-billing/nodes", infrabillingHandler.UpdateNode)
		protected.Delete("/api/infra-billing/nodes/{uuid}", infrabillingHandler.DeleteNode)
		protected.Get("/api/infra-billing/history", infrabillingHandler.GetHistory)
		protected.Post("/api/infra-billing/history", infrabillingHandler.CreateHistory)
		protected.Delete("/api/infra-billing/history/{uuid}", infrabillingHandler.DeleteHistory)

		stubHandler.RegisterRoutes(protected)
	})

	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".js", "application/javascript")

	staticDirs := []string{"./frontend/dist", "./dist", "../frontend/dist", "./static", "/opt/gowave/frontend/dist", "/opt/gowave/dist"}
	var staticDir string
	for _, dir := range staticDirs {
		if _, err := os.Stat(dir); err == nil {
			staticDir = dir
			break
		}
	}
	if staticDir != "" {
		fs := http.FileServer(http.Dir(staticDir))
		spa := func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(staticDir, r.URL.Path)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if strings.HasSuffix(r.URL.Path, ".html") {
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				}
				fs.ServeHTTP(w, r)
				return
			}

			// If requested with /assets/ prefix, check directly in root of staticDir:
			// (handles schemas like /assets/xray.schema.json, /assets/main.wasm, /assets/wasm_exec.js)
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				altPath := filepath.Join(staticDir, strings.TrimPrefix(r.URL.Path, "/assets/"))
				if info, err := os.Stat(altPath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, altPath)
					return
				}
			}

			// If an asset / file was requested (has file extension), do NOT return index.html
			ext := filepath.Ext(r.URL.Path)
			if ext != "" && ext != ".html" {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		}
		r.Get("/*", spa)
		r.Head("/*", spa)
	}

	return &App{Router: r, wait: subHandler.WaitHistoryWriter, start: func(ctx context.Context) {
		go subHandler.RunHistoryWriter(ctx)
		go notifier.CheckTelegram(ctx)
		if err := notify.Emit(db, notify.Event{Name: "service.panel_started", Data: map[string]string{"panelVersion": strings.TrimPrefix(system.AppVersion, "v")}}); err != nil {
			fmt.Printf("panel_started notification: %v\n", err)
		}
		go scheduler.Start(ctx)
		go workQueue.Start(ctx)
		go nodeService.StartHealthCheckLoop(ctx)
		go nodeService.SyncAllUsersToConnectedNodes()
		go metricsExporter.Run(ctx)
		if cfg.MetricsPort != "" {
			go prometheus.Serve(ctx, net.JoinHostPort(cfg.AppHost, cfg.MetricsPort), prometheus.Handler(db, cfg.MetricsUser, cfg.MetricsPass))
		}
	}}
}
