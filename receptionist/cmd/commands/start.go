package commands

import (
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/app"
	"github.com/fedor-malyshkin/library-simulator/receptionist/pkg/receptionist/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	RunE: func(c *cobra.Command, args []string) error {
		configPath, _ := c.Flags().GetString("config")

		cfg, err := config.FindConfig(configPath)
		if err != nil {
			return err
		}

		app, err := app.NewApp(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("!!! failed to init Server !!!")
			return err
		}

		return app.StartApp()
	},
}
