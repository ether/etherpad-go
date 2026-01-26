package settings

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func HandleConfigCommand(logger *zap.SugaredLogger) {
	if len(os.Args) < 3 {
		printConfigHelp()
		os.Exit(1)
	}

	ApplyRegistryDefaults()
	switch os.Args[2] {
	case "show":
		InitSettings(logger)
		configShow()
	case "dump":
		configDump()
	case "env":
		configEnv()
	case "get":
		configGet(os.Args[3:])
	case "init":
		configInit()
	default:
		fmt.Println("Unknown config command:", os.Args[2])
		printConfigHelp()
		os.Exit(1)
	}
	os.Exit(0)
}

func configShow() {
	fmt.Printf(
		"%-35s %-35s %-20s %-20s %s\n",
		"JSON KEY",
		"ENV VAR",
		"CURRENT",
		"DEFAULT",
		"DESCRIPTION",
	)

	for _, c := range Registry {
		fmt.Printf(
			"%-35s %-35s %-20v %-20v %s\n",
			c.Key,
			EnvVar(c.Key),
			viper.Get(c.Key),
			c.Default,
			c.Description,
		)
	}
}

func configDump() {
	all := viper.AllSettings()

	out, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(out))
}

func configEnv() {
	fmt.Printf("%-35s %s\n", "ENV VAR", "JSON KEY")

	for _, c := range Registry {
		fmt.Printf(
			"%-35s %s\n",
			EnvVar(c.Key),
			c.Key,
		)
	}
}

func configGet(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: etherpad config get <json-key>")
		return
	}

	key := args[0]

	for _, c := range Registry {
		if c.Key == key {
			fmt.Println(viper.Get(key))
			return
		}
	}

	fmt.Println("Unknown config key:", key)
}

func configInit() {
	out := map[string]any{}

	for _, c := range Registry {
		out[c.Key] = c.Default
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(b))
}

func printConfigHelp() {
	fmt.Println(`Usage:
  etherpad config show
  etherpad config dump
  etherpad config env
  etherpad config get <json-key>
  etherpad config init`)
}
