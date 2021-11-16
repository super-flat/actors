package cmd

import (
	"github.com/spf13/cobra"
	"github.com/super-flat/actors/engine"
)

func init() {
	rootCmd.AddCommand(runCMD)
}

var runCMD = &cobra.Command{
	Use: "run",
	Run: func(cmd *cobra.Command, args []string) {
		engine.Sample()
	},
}
