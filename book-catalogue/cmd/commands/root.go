package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	SilenceUsage: true,
	Run: func(c *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.PersistentFlags().String("config", "", "Config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed: %s\n", err)
		os.Exit(1)
	}
}
