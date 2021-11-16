package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCMD)
}

var runCMD = &cobra.Command{
	Use: "run",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("run")
	},
}
