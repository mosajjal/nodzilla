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
	"github.com/mosajjal/nodzilla/pkg/api"
	"github.com/mosajjal/nodzilla/pkg/db"
	"github.com/rs/zerolog"

	"github.com/spf13/cobra"
)

var nocolorLog = strings.ToLower(os.Getenv("NO_COLOR")) == "true"
var logger = zerolog.New(os.Stderr).With().Timestamp().Logger().
	Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: nocolorLog})

var (
	commit  string = "NOT_PROVIDED"
	version string = "UNKNOWN"
)

//go:embed config.defaults.yaml
var defaultConfig []byte

func main() {

	cmd := &cobra.Command{
		Use:   "nodzilla",
		Short: "nodzilla API",
		Long:  `nodzilla provides a ReST API for newly observed domains`,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}
	flags := cmd.Flags()

	logLevel := flags.StringP("loglevel", "l", "info", "log level (debug, info, warn, error, fatal, panic)")
	config := flags.StringP("config", "c", "$HOME/.nodzilla.yaml", "path to YAML configuration file")
	_ = flags.BoolP("defaultconfig", "d", false, "write default config to $HOME/.nodzilla.yaml")
	_ = flags.BoolP("version", "V", false, "print version and exit")

	if err := cmd.Execute(); err != nil {
		logger.Error().Msgf("failed to execute command: %s", err)
		return
	}

	// set up log level
	if lvl, err := zerolog.ParseLevel(*logLevel); err != nil {
		logger.Fatal().Msgf("failed to parse log level: %s", err)
	} else {
		zerolog.SetGlobalLevel(lvl)
	}

	if !flags.Changed("config") {
		if home, err := os.UserHomeDir(); err != nil {
			logger.Fatal().Msgf("failed to get user home directory: %s", err)
		} else {
			*config = filepath.Join(home, ".nodzilla.yaml")
		}
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

	// set up the database
	dbConfYAML := k.Cut("db")
	if engine := dbConfYAML.String("engine"); engine != "pebble" {
		logger.Fatal().Msgf("unsupported database engine: %s", engine)
	}
	myDB := db.NewPebbleDB(dbConfYAML.String("uri"))
	if err := myDB.Open(); err != nil {
		logger.Fatal().Msgf("failed to open database: %s", err)
	}
	defer myDB.Close()

	// set up the API
	apiConfYAML := k.Cut("api")
	apiConf := api.Config{
		ListenAddr:      apiConfYAML.String("listen"),
		BasePath:        apiConfYAML.String("base_path_api"),
		BasePathAdmin:   apiConfYAML.String("base_path_admin"),
		IsTLS:           apiConfYAML.Bool("tls_enabled"),
		TLSCert:         apiConfYAML.String("tls_cert"),
		TLSKey:          apiConfYAML.String("tls_key"),
		AuthMethodAPI:   apiConfYAML.String("auth_method_api"),
		AuthUsersAPI:    apiConfYAML.StringMap("auth_users_api"),
		AuthMethodAdmin: apiConfYAML.String("auth_method_admin"),
		AuthUsersAdmin:  apiConfYAML.StringMap("auth_users_admin"),
		Logger:          &logger,
		RPS:             apiConfYAML.Float64("rps"),
	}
	myAPI := api.NewAPI(apiConf, myDB)
	// Blocking call
	myAPI.ListenAndServe()
}
