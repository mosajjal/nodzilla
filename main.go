package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	nodzilla "github.com/mosajjal/nodzilla/pkg"
	"github.com/rs/zerolog"

	"github.com/spf13/cobra"
)

var nocolorLog = strings.ToLower(os.Getenv("NO_COLOR")) == "true"
var logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: nocolorLog})

var (
	version string = "UNKNOWN"
	commit  string = "NOT_PROVIDED"
)

//go:embed config.defaults.yaml
var defaultConfig []byte

func main() {

	cmd := &cobra.Command{
		Use:   "nodzilla",
		Short: "nodzilla is awesome",
		Long:  `nodzilla is the best CLI ever!`,
		Run: func(cmd *cobra.Command, args []string) {
			nodzilla.Run()
		},
	}
	flags := cmd.Flags()

	// define cli arguments
	_ = flags.IntP("number", "n", 7, "What is the magic number?")
	// make it required
	_ = cmd.MarkFlagRequired("number")
	logLevel := flags.StringP("loglevel", "v", "info", "log level (debug, info, warn, error, fatal, panic)")
	config := flags.StringP("config", "c", "$HOME/.nodzilla.yaml", "path to YAML configuration file")
	_ = flags.BoolP("defaultconfig", "d", false, "write default config to $HOME/.nodzilla.yaml")

	if err := cmd.Execute(); err != nil {
		logger.Error().Msgf("failed to execute command: %s", err)
		return
	}

	// set up log level
	lvl, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal().Msgf("failed to parse log level: %s", err)
	}
	zerolog.SetGlobalLevel(lvl)

	if !flags.Changed("config") {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Fatal().Msgf("failed to get user home directory: %s", err)
		}
		*config = filepath.Join(home, ".nodzilla.yaml")
	}
	if flags.Changed("help") {
		return
	}
	if flags.Changed("version") {
		fmt.Printf("nodzilla version %s, commit %s\n", version, commit)
		return
	}

	// load the default config
	if flags.Changed("defaultconfig") {
		err := ioutil.WriteFile(*config, defaultConfig, 0644)
		if err != nil {
			logger.Fatal().Msgf("failed to write default config: %s", err)
		}
		logger.Info().Msgf("wrote default config to %s", *config)
		return
	}

	k := koanf.New(".")
	// load the defaults first, so if the config file is missing some values, we can fall back to the defaults
	if err := k.Load(rawbytes.Provider(defaultConfig), yaml.Parser()); err != nil {
		logger.Fatal().Msgf("failed to load default config: %s", err)
	}

	if err := k.Load(file.Provider(*config), yaml.Parser()); err != nil {
		logger.Fatal().Msgf("failed to load config file: %s", err)
	}

	// TODO: write the rest of your app logic here

}
