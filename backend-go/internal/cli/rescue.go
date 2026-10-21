package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"gowave/config"
	"gowave/internal/auth"
	"gowave/internal/database"
	"gowave/internal/keygen"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func IsMigrationCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "--migrate-pg-to-sqlite", "migrate-pg-to-sqlite", "--migrate-sqlite-to-pg", "migrate-sqlite-to-pg", "--migrate-to-docker":
		return true
	case "migrate", "--migrate":
		if len(args) > 1 {
			sub := args[1]
			return sub == "pg-to-sqlite" || sub == "postgres-to-sqlite" || sub == "sqlite-to-pg" || sub == "sqlite-to-postgres" || sub == "to-docker"
		}
		return false
	default:
		return false
	}
}

func IsPostgresToSQLiteMigrationCommand(args []string) bool {
	return IsMigrationCommand(args)
}

func migrationDestination(args []string, index int) string {
	if len(args) > index && strings.TrimSpace(args[index]) != "" {
		return strings.TrimSpace(args[index])
	}
	return "gowave-migrated.sqlite"
}

func runToDocker(args []string) {
	if err := RunMigrateToDocker(args, bufio.NewReader(os.Stdin)); err != nil {
		fmt.Printf("[-] Migration to Docker failed: %v\n", err)
		os.Exit(1)
	}
}

// runSQLiteToPG handles "[source] [dsn] [--force]" for the SQLite -> PostgreSQL command.
func runSQLiteToPG(cfg *config.Config, args []string) {
	force := false
	var positional []string
	for _, arg := range args {
		if arg == "--force" {
			force = true
			continue
		}
		positional = append(positional, arg)
	}
	source, target := "", ""
	if len(positional) > 0 {
		source = positional[0]
	}
	if len(positional) > 1 {
		target = positional[1]
	}
	if source == "" {
		detected, err := DetectGoWaveSQLitePath(cfg)
		if err != nil {
			fmt.Printf("[-] %v\n", err)
			os.Exit(1)
		}
		source = detected
	}
	if target == "" {
		detected, err := DetectPostgresTargetDSN(cfg)
		if err != nil {
			fmt.Printf("[-] %v\n", err)
			os.Exit(1)
		}
		target = detected
	}
	if err := MigrateSQLiteToPostgres(cfg, source, target, force); err != nil {
		fmt.Printf("[-] Migration failed: %v\n", err)
		os.Exit(1)
	}
}

func RunRescue(db *gorm.DB, cfg *config.Config, args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "--reset-admin":
			ResetSuperadmin(db)
			return
		case "--list-admins":
			ListAdmins(db)
			return
		case "--enable-password-auth":
			EnablePasswordAuth(db)
			return
		case "--print-secret-key":
			PrintNodeSecretKey(db)
			return
		case "--import-pasarguard", "import-pasarguard":
			source := ""
			if len(args) > 1 {
				source = args[1]
			}
			if err := ImportFromPasarGuard(db, source); err != nil {
				fmt.Printf("[-] PasarGuard import failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "import":
			source := ""
			if len(args) > 1 && (args[1] == "pasarguard" || args[1] == "pasar") {
				if len(args) > 2 {
					source = args[2]
				}
			} else if len(args) > 1 {
				source = args[1]
			}
			if err := ImportFromPasarGuard(db, source); err != nil {
				fmt.Printf("[-] PasarGuard import failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--migrate-pg-to-sqlite", "migrate-pg-to-sqlite":
			dsn := ""
			if len(args) > 1 {
				dsn = args[1]
			}
			dest := migrationDestination(args, 2)
			if err := MigratePostgresToSQLite(cfg, dsn, dest); err != nil {
				fmt.Printf("[-] Migration failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--migrate-sqlite-to-pg", "migrate-sqlite-to-pg":
			runSQLiteToPG(cfg, args[1:])
			return
		case "--migrate-to-docker":
			runToDocker(args[1:])
			return
		case "migrate", "--migrate":
			if len(args) > 1 && (args[1] == "sqlite-to-pg" || args[1] == "sqlite-to-postgres") {
				runSQLiteToPG(cfg, args[2:])
				return
			}
			if len(args) > 1 && args[1] == "to-docker" {
				runToDocker(args[2:])
				return
			}

			dsn := ""
			argOffset := 1
			if len(args) > 1 && (args[1] == "pg-to-sqlite" || args[1] == "postgres-to-sqlite") {
				argOffset = 2
			}
			if len(args) > argOffset {
				dsn = args[argOffset]
			}
			dest := migrationDestination(args, argOffset+1)
			if err := MigratePostgresToSQLite(cfg, dsn, dest); err != nil {
				fmt.Printf("[-] Migration failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--help", "-h":
			printUsage()
			return
		}
	}

	interactiveMenu(db, cfg)
}

func PrintNodeSecretKey(db *gorm.DB) {
	kg := keygen.NewService(db)
	key, err := kg.GenerateNodeSecretKey()
	if err != nil {
		fmt.Printf("[-] Failed to generate node secret key: %v\n", err)
		return
	}
	fmt.Printf("[+] Node SECRET_KEY:\n%s\n", key)
}

func printUsage() {
	fmt.Println("Usage: gowave-server rescue [flag]")
	fmt.Println("       gowave-server import pasarguard [path/dsn]")
	fmt.Println("       gowave-server migrate pg-to-sqlite [dsn] [destination]")
	fmt.Println("       gowave-server migrate to-docker [--dir /opt/gowave-docker] [--image ref] [--yes]")
	fmt.Println("       gowave-server migrate sqlite-to-pg [source_sqlite] [target_dsn] [--force]")
	fmt.Println("       gowave-server cli [flag]")
	fmt.Println("\nAvailable commands / flags:")
	fmt.Println("  --import-pasarguard [path]       Import users, traffic and HWIDs from PasarGuard (SQLite or Postgres)")
	fmt.Println("  import pasarguard [path]         Import users, traffic and HWIDs from PasarGuard")
	fmt.Println("  --migrate-pg-to-sqlite [dsn] [dest]  Migrate upstream panel PostgreSQL data to SQLite")
	fmt.Println("  migrate pg-to-sqlite [dsn] [dest]    Migrate upstream panel PostgreSQL data to SQLite")
	fmt.Println("  migrate to-docker                    Move this SQLite install to Docker + PostgreSQL")
	fmt.Println("      --dir <path>                     Compose project directory (default /opt/gowave-docker)")
	fmt.Println("      --image <ref>                    Panel image (default ghcr.io/aaasazan/gowave:latest)")
	fmt.Println("      --from <path>                    Current install directory (default: this binary's)")
	fmt.Println("      --yes                            Do not ask for confirmation")
	fmt.Println("  --migrate-sqlite-to-pg [src] [dsn]   Copy GoWave SQLite data into an existing PostgreSQL")
	fmt.Println("  migrate sqlite-to-pg [src] [dsn]     Copy GoWave SQLite data into an existing PostgreSQL")
	fmt.Println("      --force                         Replace existing users, nodes and hosts in the target")
	fmt.Println("  --reset-admin                   Remove all admins to re-trigger first-time web onboarding")
	fmt.Println("  --list-admins                   List all existing administrators")
	fmt.Println("  --enable-password-auth          Force-enable password authentication in database")
	fmt.Println("  --print-secret-key              Generate and print a valid SECRET_KEY for GoWave Node")
	fmt.Println("  --help, -h                      Show this help message")
	fmt.Println("\nRun without flags to start the interactive rescue menu.")
}

func interactiveMenu(db *gorm.DB, cfg *config.Config) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\n==========================================")
		fmt.Println("          GoWave - Rescue CLI             ")
		fmt.Println("==========================================")
		fmt.Println("1) Reset superadmin (removes admin to re-trigger initial setup)")
		fmt.Println("2) Reset superadmin password")
		fmt.Println("3) Create new admin account")
		fmt.Println("4) List all admins")
		fmt.Println("5) Enable username/password authentication")
		fmt.Println("6) Print node SECRET_KEY")
		fmt.Println("7) Import database from PasarGuard (SQLite/PostgreSQL)")
		fmt.Println("8) Migrate from upstream panel PostgreSQL -> Gowave SQLite (and purge Docker)")
		fmt.Println("9) Move Gowave to Docker (SQLite -> PostgreSQL)")
		fmt.Println("0) Exit")
		fmt.Print("\nSelect an option [0-9]: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			ResetSuperadmin(db)
		case "2":
			resetAdminPasswordInteractive(db, cfg, reader)
		case "3":
			createAdminInteractive(db, cfg, reader)
		case "4":
			ListAdmins(db)
		case "5":
			EnablePasswordAuth(db)
		case "6":
			PrintNodeSecretKey(db)
		case "7":
			ImportPasarGuardInteractive(db, reader)
		case "8":
			MigratePGToSQLiteInteractive(cfg, reader)
		case "9":
			if err := RunMigrateToDocker(nil, reader); err != nil {
				fmt.Printf("[-] Migration to Docker failed: %v\n", err)
			}
		case "0", "q", "exit":
			fmt.Println("Exiting Rescue CLI.")
			return
		default:
			fmt.Println("Invalid option, please choose between 0 and 9.")
		}
	}
}

func ResetSuperadmin(db *gorm.DB) {
	result := db.Exec("DELETE FROM admin")
	if result.Error != nil {
		fmt.Printf("[-] Failed to reset superadmin: %v\n", result.Error)
		return
	}
	fmt.Printf("[+] Superadmin account(s) deleted (%d removed).\n", result.RowsAffected)
	fmt.Println("[+] Open http://<server-ip>:3333 in your browser to complete initial setup!")
}

func ListAdmins(db *gorm.DB) {
	var admins []database.Admin
	if err := db.Find(&admins).Error; err != nil {
		fmt.Printf("[-] Failed to query admins: %v\n", err)
		return
	}
	if len(admins) == 0 {
		fmt.Println("[i] No admins found in database. Initial setup wizard is available in web UI.")
		return
	}
	fmt.Printf("[+] Found %d admin(s):\n", len(admins))
	for i, a := range admins {
		fmt.Printf("  %d. Username: %s (UUID: %s, Role: %s)\n", i+1, a.Username, a.UUID, a.Role)
	}
}

func EnablePasswordAuth(db *gorm.DB) {
	res := db.Model(&database.PanelSetting{}).Where("id = 1").Updates(map[string]interface{}{
		"password_auth_enabled": true,
	})
	if res.Error != nil {
		fmt.Printf("[-] Failed to enable password authentication: %v\n", res.Error)
		return
	}
	fmt.Println("[+] Password authentication has been forcibly enabled!")
}

func resetAdminPasswordInteractive(db *gorm.DB, cfg *config.Config, reader *bufio.Reader) {
	var admins []database.Admin
	db.Find(&admins)
	if len(admins) == 0 {
		fmt.Println("[-] No admins to reset. Please use option 1 to trigger initial setup or option 3 to create an admin.")
		return
	}

	fmt.Print("Enter username of the admin to update: ")
	userIn, _ := reader.ReadString('\n')
	username := strings.TrimSpace(userIn)

	var admin database.Admin
	if err := db.Where("username = ?", username).First(&admin).Error; err != nil {
		fmt.Printf("[-] Admin '%s' not found.\n", username)
		return
	}

	fmt.Print("Enter new password (min 8 chars): ")
	passIn, _ := reader.ReadString('\n')
	password := strings.TrimSpace(passIn)
	if len(password) < 8 {
		fmt.Println("[-] Password must be at least 8 characters.")
		return
	}

	hash, err := auth.HashPassword(password, cfg.AppSecret)
	if err != nil {
		fmt.Printf("[-] Failed to hash password: %v\n", err)
		return
	}

	admin.PasswordHash = hash
	admin.UpdatedAt = time.Now()
	if err := db.Save(&admin).Error; err != nil {
		fmt.Printf("[-] Failed to save password: %v\n", err)
		return
	}

	fmt.Printf("[+] Password for admin '%s' updated successfully!\n", username)
}

func createAdminInteractive(db *gorm.DB, cfg *config.Config, reader *bufio.Reader) {
	fmt.Print("Enter new admin username: ")
	userIn, _ := reader.ReadString('\n')
	username := strings.TrimSpace(userIn)
	if username == "" {
		fmt.Println("[-] Username cannot be empty.")
		return
	}

	fmt.Print("Enter password (min 8 chars): ")
	passIn, _ := reader.ReadString('\n')
	password := strings.TrimSpace(passIn)
	if len(password) < 8 {
		fmt.Println("[-] Password must be at least 8 characters.")
		return
	}

	hash, err := auth.HashPassword(password, cfg.AppSecret)
	if err != nil {
		fmt.Printf("[-] Failed to hash password: %v\n", err)
		return
	}

	newAdmin := database.Admin{
		UUID:         uuid.NewString(),
		Username:     username,
		PasswordHash: hash,
		Role:         "ADMIN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.Create(&newAdmin).Error; err != nil {
		fmt.Printf("[-] Failed to create admin: %v\n", err)
		return
	}

	fmt.Printf("[+] Admin '%s' created successfully!\n", username)
}
