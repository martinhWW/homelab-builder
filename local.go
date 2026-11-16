package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

var launch = launchInstance
var verify = verifyInstance
var verifyK3s = verifyK3sExists
var installK3s = installK3sOnVm

func createLocal(w io.Writer, name string) error {
	// Check if Multipass is installed on the host machine
	output, err := checkMultipass()
	if err != nil {
		return err
	}

	// Check if a Multipass VM instance exists with the provided name
	exists, err := instanceExists(output, name)
	if err != nil {
		return err
	}
	if exists {
		fmt.Fprintf(w, "vm %q already exists, skipping launch\n", name)
	} else {
		// Launch new Multipass VM instance
		launchArgs := buildMultipassLaunchArgs(name)
		fmt.Fprintf(w, "multipass %s\n", strings.Join(launchArgs, " "))
		if err := launch(w, launchArgs); err != nil {
			return err
		}
	}
	// Verify VM instance is reachable
	if err := verify(name); err != nil {
		return err
	}
	fmt.Fprintf(w, "vm %q is ready\n", name)

	// Check if K3s is already installed on VM instance
	installed, err := verifyK3s(name)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(w, "k3s is already installed on VM %q\n", name)
		return nil
	}
	err = installK3s(w, name)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "k3s has been installed on VM %q\n", name)

	return nil
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

func installK3sOnVm(w io.Writer, name string) error {
	downloadCmd := `curl -sfL https://get.k3s.io -o /tmp/k3s-install.sh`
	installCmd := `INSTALL_K3S_EXEC="server --disable traefik --write-kubeconfig-mode 644" sh /tmp/k3s-install.sh`

	// Pass "sh", "-c", and the full command string as separate arguments to multipass exec
	cmd := exec.Command("multipass", "exec", name, "--", "sh", "-c", downloadCmd+" && "+installCmd)

	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install k3s on %q: %w", name, err)
	}

	return nil
}

func verifyK3sExists(name string) (bool, error) {
	_, err := exec.Command("multipass", "exec", name, "--", "which", "k3s").Output()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("checking for k3s on VM %q: %w", name, err)
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
