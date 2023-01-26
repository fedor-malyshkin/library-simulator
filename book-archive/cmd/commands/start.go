package commands

import (
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/app"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
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
			log.Fatal().Err(err).Msg("!!! failed to start Server !!!")
			return err
		}

		return app.Run()
	},
}
