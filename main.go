package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var version string
var launch = launchInstance
var verify = verifyInstance

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
			if err := validateCreateInputs(provider, numNodes, environmentName); err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), createHeader(provider, numNodes, environmentName))

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				output, err := checkMultipass()
				if err != nil {
					return err
				}
				exists, err := instanceExists(output, environmentName)
				if err != nil {
					return err
				}
				if exists {
					return fmt.Errorf("homelab instance %q already exists: choose a different name with --name", environmentName)
				}

				launchArgs := buildMultipassLaunchArgs(environmentName)
				fmt.Fprintf(cmd.OutOrStdout(), "multipass %s\n", strings.Join(launchArgs, " "))
				err = launch(cmd.OutOrStdout(), launchArgs)
				if err != nil {
					return err
				}
				err = verify(environmentName)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "vm %q is ready\n", environmentName)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().IntVarP(&numNodes, "nodes", "n", 1, "The number of nodes to create for the homelab environment")
	cmd.Flags().StringVar(&environmentName, "name", "homelab", "The name of the lab environment")
	return cmd
}

func checkMultipass() ([]byte, error) {
	_, err := exec.LookPath("multipass")
	if err != nil {
		return nil, fmt.Errorf("multipass not found: %w: install it from https://multipass.run/install", err)
	}
	output, err := exec.Command("multipass", "list", "--format", "json").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("command 'multipass list' failed: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("command 'multipass list' failed: %w", err)
	}
	return output, nil
}

func launchInstance(w io.Writer, args []string) error {
	c := exec.Command("multipass", args...)
	c.Stdout = w
	c.Stderr = w
	if err := c.Run(); err != nil {
		return fmt.Errorf("multipass launch failed: %w", err)
	}
	return nil
}

func verifyInstance(name string) error {
	_, err := exec.Command("multipass", "exec", name, "--", "true").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("connection to VM %q failed: %w: %s", name, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("connection to VM %q failed: %w", name, err)
	}
	return nil
}

func buildMultipassLaunchArgs(name string) []string {
	return []string{
		"launch",
		"24.04",
		"--name",
		name,
		"--cpus",
		"2",
		"--memory",
		"4G",
		"--disk",
		"30G",
	}
}

func validateCreateInputs(provider string, nodes int, name string) error {
	if provider != "local" && provider != "aws" && provider != "gcp" {
		return fmt.Errorf("invalid provider: %s: must be aws, gcp, or local", provider)
	}
	if nodes < 1 || nodes > 15 {
		return fmt.Errorf("invalid number of nodes: %d: must be greater than 0 and no greater than 15", nodes)
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid name: %q: must start with a lowercase letter, end with a lowercase letter or digit, contain only lowercase letters, digits, or hyphens, and be at most 63 characters", name)
	}
	return nil
}

func createHeader(provider string, nodes int, name string) string {
	return fmt.Sprintf("Creating a new homelab environment: %s\nProvider: %s\nNodes: %d\n", name, provider, nodes)
}

func instanceExists(data []byte, name string) (bool, error) {
	var list MultipassList
	if err := json.Unmarshal(data, &list); err != nil {
		return false, fmt.Errorf("parsing multipass list output: %w", err)
	}
	for _, instance := range list.Instances {
		if instance.Name == name {
			return true, nil
		}
	}
	return false, nil
}

type MultipassList struct {
	Instances []MultipassInstance `json:"list"`
}

type MultipassInstance struct {
	Name string `json:"name"`
}

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
