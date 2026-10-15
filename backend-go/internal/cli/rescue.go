package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/auth"
	"remnawave-go/internal/database"
	"remnawave-go/internal/keygen"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func IsPostgresToSQLiteMigrationCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "--migrate-pg-to-sqlite", "migrate-pg-to-sqlite":
		return true
	case "migrate":
		return len(args) > 1 && (args[1] == "pg-to-sqlite" || args[1] == "postgres-to-sqlite")
	case "--migrate":
		return len(args) > 1 && (args[1] == "pg-to-sqlite" || args[1] == "postgres-to-sqlite")
	default:
		return false
	}
}

func migrationDestination(args []string, index int) string {
	if len(args) > index && strings.TrimSpace(args[index]) != "" {
		return strings.TrimSpace(args[index])
	}
	return "gowave-migrated.sqlite"
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
		case "migrate", "--migrate":
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
	fmt.Println("Usage: remnawave-server rescue [flag]")
	fmt.Println("       remnawave-server import pasarguard [path/dsn]")
	fmt.Println("       remnawave-server migrate pg-to-sqlite [dsn] [destination]")
	fmt.Println("       remnawave-server cli [flag]")
	fmt.Println("\nAvailable commands / flags:")
	fmt.Println("  --import-pasarguard [path]       Import users, traffic and HWIDs from PasarGuard (SQLite or Postgres)")
	fmt.Println("  import pasarguard [path]         Import users, traffic and HWIDs from PasarGuard")
	fmt.Println("  --migrate-pg-to-sqlite [dsn] [destination]  Migrate Remnawave PostgreSQL data to SQLite")
	fmt.Println("  migrate pg-to-sqlite [dsn] [destination]    Migrate Remnawave PostgreSQL data to SQLite")
	fmt.Println("  --reset-admin                   Remove all admins to re-trigger first-time web onboarding")
	fmt.Println("  --list-admins                   List all existing administrators")
	fmt.Println("  --enable-password-auth          Force-enable password authentication in database")
	fmt.Println("  --print-secret-key              Generate and print a valid SECRET_KEY for remnanode")
	fmt.Println("  --help, -h                      Show this help message")
	fmt.Println("\nRun without flags to start the interactive rescue menu.")
}

func interactiveMenu(db *gorm.DB, cfg *config.Config) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\n==========================================")
		fmt.Println("       Remnawave Go - Rescue CLI          ")
		fmt.Println("==========================================")
		fmt.Println("1) Reset superadmin (removes admin to re-trigger initial setup)")
		fmt.Println("2) Reset superadmin password")
		fmt.Println("3) Create new admin account")
		fmt.Println("4) List all admins")
		fmt.Println("5) Enable username/password authentication")
		fmt.Println("6) Print node SECRET_KEY")
		fmt.Println("7) Import database from PasarGuard (SQLite/PostgreSQL)")
		fmt.Println("8) Migrate from Remnawave PostgreSQL -> Gowave SQLite (and purge Docker)")
		fmt.Println("0) Exit")
		fmt.Print("\nSelect an option [0-8]: ")

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
		case "0", "q", "exit":
			fmt.Println("Exiting Rescue CLI.")
			return
		default:
			fmt.Println("Invalid option, please choose between 0 and 8.")
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
	res := db.Model(&database.RemnawaveSetting{}).Where("id = 1").Updates(map[string]interface{}{
		"password_auth_enabled": true,
		"password_settings":     `{"enabled":true}`,
	})
	if res.Error != nil {
		fmt.Printf("[-] Failed to update settings: %v\n", res.Error)
		return
	}
	fmt.Println("[+] Password authentication has been re-enabled!")
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
