package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// ======================================================================
// Globals and types
//
// Build-time version, name validation, and the per-homelab state record.
// ======================================================================

var version string

// validName matches a name every provider accepts (Multipass, AWS, GCP): starts
// with a lowercase letter, ends with a lowercase letter or digit, only lowercase
// letters, digits, and hyphens in between, at most 63 characters.
var validName = regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)

type HomelabState struct {
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Nodes     int       `json:"nodes"`
	CreatedAt time.Time `json:"createdAt"`
}

// ======================================================================
// Entry point
//
// Runs the root command and prints any error.
// ======================================================================

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
		os.Exit(1)
	}
}

// ======================================================================
// Commands
//
// Cobra command definitions: root, version, create, status, list, destroy.
// ======================================================================

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
	rootCmd.AddCommand(newDestroyCommand())
	rootCmd.AddCommand(newListCommand())
	return rootCmd
}

func getVersion() string {
	if version == "" {
		return "Homelab dev"
	}
	return "Homelab v" + version
}

func newCreateCommand() *cobra.Command {
	var provider string
	var numNodes int
	var envName string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new homelab environment",
		Long:  "Create a new homelab environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNameFlag(envName); err != nil {
				return err
			}
			if err := validateProviderFlag(provider); err != nil {
				return err
			}
			if err := validateNodesFlag(numNodes); err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), createHeader(provider, numNodes, envName))

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				return createLocal(cmd.OutOrStdout(), envName)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().IntVarP(&numNodes, "nodes", "n", 1, "The number of nodes to create for the homelab environment")
	cmd.Flags().StringVar(&envName, "name", "homelab", "The name of the lab environment")
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
			fmt.Fprintf(cmd.OutOrStdout(), "→ Checking homelab %q...\n  Name:      %s\n  Provider:  %s\n", name, name, provider)

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

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all existing homelab environments",
		Long:  "Returns a list of all existing homelab environments regardless of state",
		RunE: func(cmd *cobra.Command, args []string) error {

			currentStateList, err := listStates()
			if err != nil {
				return err
			}
			if len(currentStateList) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "• No homelabs found")
				return nil
			}

			instancesList, err := listInstances()
			if err != nil {
				return err
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tPROVIDER\tSTATE\tNODES")

			for _, val := range currentStateList {
				instance, found, err := findInstance(instancesList, val.Name)
				if err != nil {
					return err
				}
				state := "Not found"

				if found {
					state = instance.State
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", val.Name, val.Provider, state, val.Nodes)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newDestroyCommand() *cobra.Command {
	var provider string
	var name string
	var force bool
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy a homelab environment",
		Long:  "Destroy a homelab environment: delete its VM and remove its local files, including the kubeconfig",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNameFlag(name); err != nil {
				return err
			}
			if err := validateProviderFlag(provider); err != nil {
				return err
			}

			if !force {
				confirmed, err := confirmDestroy(cmd.InOrStdin(), cmd.OutOrStdout(), name)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "• Destroy aborted, nothing was changed")
					return nil
				}
			}

			switch provider {
			case "aws":
			case "gcp":
			case "local":
				return destroyLocal(cmd.OutOrStdout(), name)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "local", "The provider to use for the homelab environment (aws, gcp, local)")
	cmd.Flags().StringVar(&name, "name", "homelab", "The name of the lab environment")
	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt")
	return cmd
}

// ======================================================================
// Flag validation
//
// Checks user-supplied flags before any provider work runs.
// ======================================================================

func validateNameFlag(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid name: %q: must start with a lowercase letter, end with a lowercase letter or digit, contain only lowercase letters, digits, or hyphens, and be at most 63 characters", name)
	}
	return nil
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

// ======================================================================
// Output and prompts
//
// User-facing headers and the destroy confirmation prompt.
// ======================================================================

func createHeader(provider string, nodes int, name string) string {
	return fmt.Sprintf("→ Creating homelab %q...\n  Name:      %s\n  Provider:  %s\n  Nodes:     %d\n", name, name, provider, nodes)
}

func confirmDestroy(r io.Reader, w io.Writer, name string) (bool, error) {
	fmt.Fprintf(w, "⚠ Destroy homelab %q? This cannot be undone. [y/N]: ", name)
	input, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	input = strings.TrimSpace(input)
	input = strings.ToLower(input)
	if input == "y" || input == "yes" {
		return true, nil
	}
	return false, nil
}

// ======================================================================
// Paths and state files
//
// Locates ~/.homelab and reads/writes each homelab's state.json and kubeconfig.
// ======================================================================

func buildHomelabRootDir() (string, error) {
	userHomepath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	homelabPath := filepath.Join(userHomepath, ".homelab")
	return homelabPath, nil

}

func envDir(name string) (string, error) {
	homelabPath, err := buildHomelabRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homelabPath, name), nil
}

func listStates() ([]HomelabState, error) {
	homelabPath, err := buildHomelabRootDir()
	if err != nil {
		return nil, err
	}

	dir, err := os.ReadDir(homelabPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading .homelab directory: %w", err)
	}

	var currentStateList []HomelabState
	for _, lab := range dir {
		if !lab.IsDir() {
			continue
		}
		labDir := filepath.Join(homelabPath, lab.Name())
		state, err := loadState(labDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		currentStateList = append(currentStateList, state)

	}
	return currentStateList, nil
}

func loadState(dir string) (HomelabState, error) {
	var homelabState HomelabState
	pathToFile := filepath.Join(dir, "state.json")
	contents, err := os.ReadFile(pathToFile)
	if err != nil {
		return HomelabState{}, fmt.Errorf("reading homelab state: %w", err)
	}
	if err = json.Unmarshal(contents, &homelabState); err != nil {
		return HomelabState{}, fmt.Errorf("parsing homelab state: %w", err)
	}
	return homelabState, nil

}

func saveState(dir string, s HomelabState) error {
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("creating environment directory: %w", err)
	}
	stateJSON, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding homelab state: %w", err)
	}
	pathToFile := filepath.Join(dir, "state.json")
	err = os.WriteFile(pathToFile, stateJSON, 0600)
	if err != nil {
		return fmt.Errorf("writing homelab state: %w", err)
	}
	return nil
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
