package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version string

func getVersion() string {
	if version == "" {
		version = "0.1.0"
	}
	return "Homelab v" + version
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "homelab",
		Short: "A CLI for creating and deploying a homelab environment",
		Long:  "A CLI for creating and deploying a homelab environment to a local Multipass instance, AWS, or GCP",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "version",
			Short: "Print the version number of Homelab",
			Long:  "Print the version number of Homelab",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Fprintln(cmd.OutOrStdout(), getVersion())
			},
		},
	)
	return rootCmd
}

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
