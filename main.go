package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	rootCmd.AddCommand(newCreateCommand())
	return rootCmd
}

func newCreateCommand() *cobra.Command {
	var provider string
	var numNodes int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new homelab environment",
		Long:  "Create a new homelab environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if numNodes < 1 || numNodes > 15 {
				return fmt.Errorf("invalid number of nodes: %d: must be greater than 0 and no greater than 15", numNodes)
			}

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				if err := checkMultipass(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid provider: %s: must be aws, gcp, or local", provider)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Creating a new homelab environment...\nProvider: %s\nNodes: %d\n", provider, numNodes)
			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().IntVarP(&numNodes, "nodes", "n", 1, "The number of nodes to create for the homelab environment")
	return cmd
}

func checkMultipass() error {
	_, err := exec.LookPath("multipass")
	if err != nil {
		return fmt.Errorf("multipass not found: %w: install it from https://multipass.run/install", err)
	}
	output, err := exec.Command("multipass", "list").CombinedOutput()
	if err != nil {
		return fmt.Errorf("command 'multipass list' failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
