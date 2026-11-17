package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var version string

// validName matches a name every provider accepts (Multipass, AWS, GCP): starts
// with a lowercase letter, ends with a lowercase letter or digit, only lowercase
// letters, digits, and hyphens in between, at most 63 characters.
var validName = regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)

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
	rootCmd.AddCommand(newStatusCommand())
	return rootCmd
}

func newCreateCommand() *cobra.Command {
	var provider string
	var numNodes int
	var environmentName string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new homelab environment",
		Long:  "Create a new homelab environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNameFlag(environmentName); err != nil {
				return err
			}
			if err := validateProviderFlag(provider); err != nil {
				return err
			}
			if err := validateNodesFlag(numNodes); err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), createHeader(provider, numNodes, environmentName))

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				return createLocal(cmd.OutOrStdout(), environmentName)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().IntVarP(&numNodes, "nodes", "n", 1, "The number of nodes to create for the homelab environment")
	cmd.Flags().StringVar(&environmentName, "name", "homelab", "The name of the lab environment")
	return cmd
}

func newStatusCommand() *cobra.Command {
	var provider string
	var name string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Print status of homelab env",
		Long:  "Print status of specified homelab environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNameFlag(name); err != nil {
				return err
			}
			if err := validateProviderFlag(provider); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Retrieving Homelab Status...\nName: %s\nProvider: %s\n", name, provider)

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				return statusLocal(cmd.OutOrStdout(), name)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().StringVar(&name, "name", "homelab", "The name of the lab environment")
	return cmd
}

func validateProviderFlag(provider string) error {
	if provider != "local" && provider != "aws" && provider != "gcp" {
		return fmt.Errorf("invalid provider: %s: must be aws, gcp, or local", provider)
	}
	if provider == "aws" || provider == "gcp" {
		return fmt.Errorf("provider %s is not yet supported", provider)
	}
	return nil
}
func validateNodesFlag(nodes int) error {
	if nodes < 1 || nodes > 15 {
		return fmt.Errorf("invalid number of nodes: %d: must be greater than 0 and no greater than 15", nodes)
	}
	return nil
}
func validateNameFlag(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid name: %q: must start with a lowercase letter, end with a lowercase letter or digit, contain only lowercase letters, digits, or hyphens, and be at most 63 characters", name)
	}
	return nil
}

func createHeader(provider string, nodes int, name string) string {
	return fmt.Sprintf("Creating a new homelab environment: %s\nProvider: %s\nNodes: %d\n", name, provider, nodes)
}

func environmentDir(name string) (string, error) {
	homepath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(homepath, ".homelab", name), nil
}

func saveKubeconfig(dir string, data []byte) (string, error) {
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return "", fmt.Errorf("creating environment directory: %w", err)
	}
	pathToFile := filepath.Join(dir, "kubeconfig")
	err = os.WriteFile(pathToFile, data, 0600)
	if err != nil {
		return "", fmt.Errorf("writing kubeconfig: %w", err)
	}
	return pathToFile, nil
}

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
