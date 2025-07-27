package app

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/lexxiebelle/wgmeshconf/internal/config"
	"github.com/lexxiebelle/wgmeshconf/internal/db"
)

// App represents the main application
type App struct {
	config     *config.Config
	configPath string
	db         *db.DB
	dbPath     string
}

func NewApp(configPath, dbPath string) *App {
	config, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
		log.Fatalf("Run 'wgmeshconf init' to create a sample config file")
	}
	db, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
		log.Fatalf("Run 'wgmeshconf init' to create database file")
	}
	return &App{
		config:     config,
		db:         db,
		dbPath:     dbPath,
		configPath: configPath,
	}
}

func (a *App) Run(args []string, additionalArgs []string) error {
	commandName := args[0]
	switch commandName {
	case "apply":
		return a.applyCommand()
	case "write":
		return a.writeCommand()
	case "status":
		showKeys := slices.Contains(additionalArgs, "showKeys")
		return a.statusCommand(showKeys)
	case "cleanup":
		return a.cleanupCommand()
	case "backup":
		return a.backupCommand()
	case "restore":
		return a.restoreCommand(args[1:])
	case "backups":
		return a.listBackupsCommand()
	default:
		return fmt.Errorf("unknown command: %s", commandName)
	}
}

func (a *App) askConfirmation(prompt string) bool {
	fmt.Print(prompt + " [N/y]: ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func ShowHelp() {
	fmt.Printf("Usage: wgmeshconf [options] <command>\n")
	fmt.Printf("\nCommands:\n")
	fmt.Printf("  init     - Initialize application (create sample config and database)\n")
	fmt.Printf("  apply    - Apply configuration changes\n")
	fmt.Printf("  generate - Generate WireGuard configuration files\n")
	fmt.Printf("  status   - Show current status\n")
	fmt.Printf("  cleanup  - Clean up database and generated files\n")
	fmt.Printf("  backup   - Create a backup of current state\n")
	fmt.Printf("  restore  - Restore from a backup\n")
	fmt.Printf("  backups  - List available backups\n")
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  -f, --file string     Path to config file (default \"config.yaml\")\n")
	fmt.Printf("  -d, --database string Path to database file (default \"wgmeshconf.db\")\n")
	fmt.Printf("  -h, --help            Show this help message\n")
	fmt.Printf("\nExamples:\n")
	fmt.Printf("  wgmeshconf -f myconfig.yaml apply\n")
	fmt.Printf("  wgmeshconf apply -f myconfig.yaml\n")
	fmt.Printf("  wgmeshconf -d custom.db status\n")
	fmt.Printf("  wgmeshconf -f config.yaml -d database.db generate\n")
}

func Init(configPath, dbPath string) error {
	fmt.Printf("Initializing wgmeshconf...\n")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Creating sample config file: %s\n", configPath)
		if err := config.CreateSample(configPath); err != nil {
			return fmt.Errorf("failed to create sample config: %w", err)
		}
	} else {
		fmt.Printf("Config file already exists: %s\n", configPath)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Creating database file: %s\n", dbPath)
	}

	if err := db.Initialize(dbPath); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	fmt.Printf("Initialization complete!\n")
	fmt.Printf("Edit %s with your configuration and run 'wgmeshconf apply'\n", configPath)
	return nil
}

func (a *App) cleanupCommand() error {
	if !a.askConfirmation("Are you sure you want to cleanup the database and generated files?") {
		fmt.Println("Operation cancelled")
		return nil
	}
	fmt.Printf("Cleaning up...\n")

	if err := a.db.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}

	if err := os.RemoveAll("configs"); err != nil {
		return fmt.Errorf("failed to remove configs directory: %w", err)
	}

	fmt.Printf("Cleanup completed successfully!\n")
	return nil
}
