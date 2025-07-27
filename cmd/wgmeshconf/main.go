package main

import (
	"log"
	"os"

	"github.com/lexxiebelle/wgmeshconf/internal/app"
	flag "github.com/spf13/pflag"
)

const (
	defaultConfigPath = "config.yaml"
	defaultDBPath     = "wgmeshconf.db"
)

func main() {
	var configPath string
	var dbPath string
	var showHelp bool
	var showKeys bool

	flag.StringVarP(&configPath, "file", "f", defaultConfigPath, "Path to config file")
	flag.StringVarP(&dbPath, "database", "d", defaultDBPath, "Path to database file")
	flag.BoolVarP(&showHelp, "help", "h", false, "Show help information")
	flag.BoolVarP(&showKeys, "showKeys", "k", false, "Show private keys in status output")

	flag.Parse()
	args := flag.Args()

	flag.CommandLine.Parse(os.Args[1:])

	var additionalArgs []string
	if showKeys {
		additionalArgs = append(additionalArgs, "showKeys")
	}

	if showHelp || len(args) == 0 {
		app.ShowHelp()
		return
	}

	if args[0] == "init" {
		app.Init(configPath, dbPath)
		return
	}

	App := app.NewApp(configPath, dbPath)
	if err := App.Run(args, additionalArgs); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
