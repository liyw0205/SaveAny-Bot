package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/api"
	"github.com/krau/SaveAny-Bot/common/logbuffer"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/database"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the visual configuration web UI",
	Run:   runWeb,
}

func init() {
	webCmd.Flags().StringP("config", "c", config.DefaultConfigFile, "config file path")
	webCmd.Flags().String("host", "0.0.0.0", "web UI listen host")
	webCmd.Flags().Int("port", config.DefaultAPIPort, "web UI listen port")
	webCmd.Flags().String("token", "", "web UI API token")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()
	logger := log.NewWithOptions(io.MultiWriter(os.Stdout, logbuffer.Default()), log.Options{
		Level:           log.InfoLevel,
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
		ReportCaller:    true,
	})
	log.SetDefault(logger)
	ctx = log.WithContext(ctx, logger)

	configPath, _ := cmd.Flags().GetString("config")
	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetInt("port")
	token, _ := cmd.Flags().GetString("token")

	if _, err := os.Stat(configPath); err == nil {
		if err := config.Init(ctx, configPath); err != nil {
			logger.Warn("Config file exists but could not be loaded; web editor will still start", "error", err)
		} else if err := database.Open(ctx); err != nil {
			logger.Warn("Database could not be opened; rule editor will become available after a valid config is saved", "error", err)
		}
	}
	if token == "" && host != "127.0.0.1" && host != "localhost" {
		logger.Warn("Config web UI is listening without a token", "host", host)
	}
	if _, err := api.StartConfigWebServer(ctx, api.ConfigWebServerOptions{
		ConfigPath: configPath,
		Host:       host,
		Port:       port,
		Token:      token,
	}); err != nil {
		logger.Fatal("Failed to start config web server", "error", err)
	}
	fmt.Printf("Config web UI: http://%s:%d/config\n", host, port)
	<-ctx.Done()
}
