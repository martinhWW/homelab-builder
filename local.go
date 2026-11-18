package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"
)

var launch = launchInstance
var verify = verifyInstance
var verifyK3s = verifyK3sExists
var installK3s = installK3sOnVm
var getVmInfo = vmInfo
var getKubeconfig = fetchKubeconfig
var waitForNodeReadyCheck = waitForNodeReady
var listInstances = checkMultipass
var checkNodeReady = nodeReady
var deleteVM = deleteInstance

func createLocal(w io.Writer, name string) error {
	// The Multipass instance name; same as the homelab name.
	vm := name
	// Check if Multipass is installed on the host machine
	output, err := listInstances()
	if err != nil {
		return err
	}

	dir, err := envDir(name)
	if err != nil {
		return err
	}

	// Check if a Multipass VM instance exists with the provided name
	exists, err := instanceExists(output, vm)
	if err != nil {
		return err
	}
	if exists {
		err = checkManagedInstance(dir, vm)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "vm %q already exists, skipping launch\n", vm)
	} else {
		// Launch new Multipass VM instance
		launchArgs := buildMultipassLaunchArgs(vm)
		fmt.Fprintf(w, "multipass %s\n", strings.Join(launchArgs, " "))
		if err := launch(w, launchArgs); err != nil {
			return err
		}
		err = saveState(dir, HomelabState{Name: name, Provider: "local", CreatedAt: time.Now()})
		if err != nil {
			return err
		}
	}

	// Verify VM instance is reachable
	if err := verify(vm); err != nil {
		return err
	}
	fmt.Fprintf(w, "vm %q is ready\n", vm)

	// Check if K3s is already installed on VM instance
	installed, err := verifyK3s(vm)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(w, "k3s is already installed on VM %q\n", vm)
	} else {
		err = installK3s(w, vm)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "k3s has been installed on VM %q\n", vm)
	}

	info, err := getVmInfo(vm)
	if err != nil {
		return err
	}
	ipString, err := parseVMIP(info, vm)
	if err != nil {
		return err
	}
	kubeconfig, err := getKubeconfig(vm)
	if err != nil {
		return err
	}
	editedKubeconfig := setKubeconfigServer(kubeconfig, ipString)
	pathToKubeconfig, err := saveKubeconfig(dir, editedKubeconfig)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "waiting for node to be ready...")
	err = waitForNodeReadyCheck(vm)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "connect to k3s cluster using kubeconfig: export KUBECONFIG=%q\n", pathToKubeconfig)

	return nil
}

func statusLocal(w io.Writer, name string) error {
	// The Multipass instance name; same as the homelab name.
	vm := name
	output, err := listInstances()
	if err != nil {
		return err
	}
	instance, found, err := findInstance(output, vm)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("homelab %q not found", name)
	}
	ip := "none"
	if len(instance.IPv4) > 0 {
		ip = instance.IPv4[0]
	}
	fmt.Fprintf(w, "State: %s\nIP: %s\n", instance.State, ip)
	if instance.State == "Running" {
		status, err := checkNodeReady(vm)
		if err != nil {
			fmt.Fprintf(w, "K3s Node Status: unknown (%v)\n", err)
			return nil
		}
		if status {
			fmt.Fprintln(w, "K3s Node Status: Ready")
		} else {
			fmt.Fprintln(w, "K3s Node Status: NotReady")
		}
	} else {
		fmt.Fprintln(w, "K3s Node Status: n/a")
	}
	return nil
}

func destroyLocal(w io.Writer, name string) error {
	vm := name
	output, err := listInstances()
	if err != nil {
		return err
	}
	dir, err := envDir(name)
	if err != nil {
		return err
	}
	exists, err := instanceExists(output, vm)
	if err != nil {
		return err
	}
	if exists {
		err = checkManagedInstance(dir, vm)
		if err != nil {
			return err
		}
		// Stop here if the delete fails: removing dir would orphan the VM.
		if err := deleteVM(vm); err != nil {
			return err
		}
		fmt.Fprintf(w, "vm %q deleted\n", vm)
	} else {
		_, err = loadState(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("homelab %q not found", name)
		} else if err != nil {
			return err
		}
		fmt.Fprintf(w, "vm %q not found, cleaning up local files\n", vm)
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("removing %q: %w", dir, err)
	}
	fmt.Fprintf(w, "removed %s\n", dir)
	return nil
}

// runMultipass runs a multipass subcommand and returns its stdout. On a
// non-zero exit, the returned error includes multipass's stderr.
func runMultipass(args ...string) ([]byte, error) {
	output, err := exec.Command("multipass", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return nil, fmt.Errorf("%w: %s", err, stderr)
			}
		}
		return nil, err
	}
	return output, nil
}

func checkMultipass() ([]byte, error) {
	_, err := exec.LookPath("multipass")
	if err != nil {
		return nil, fmt.Errorf("multipass not found: %w: install it from https://multipass.run/install", err)
	}
	output, err := runMultipass("list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("command 'multipass list' failed: %w", err)
	}
	return output, nil
}

func checkManagedInstance(dir, vm string) error {
	_, err := loadState(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("vm %q exists but was not created by homelab-builder", vm)
	}
	return err
}

func deleteInstance(vm string) error {
	_, err := runMultipass("delete", "--purge", vm)
	if err != nil {
		return fmt.Errorf("deleting VM %q: %w", vm, err)
	}
	return nil
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

func verifyInstance(vm string) error {
	if _, err := runMultipass("exec", vm, "--", "true"); err != nil {
		return fmt.Errorf("connection to VM %q failed: %w", vm, err)
	}
	return nil
}

func buildMultipassLaunchArgs(vm string) []string {
	return []string{
		"launch",
		"24.04",
		"--name",
		vm,
		"--cpus",
		"2",
		"--memory",
		"4G",
		"--disk",
		"30G",
	}
}

func installK3sOnVm(w io.Writer, vm string) error {
	downloadCmd := `curl -sfL https://get.k3s.io -o /tmp/k3s-install.sh`
	installCmd := `INSTALL_K3S_EXEC="server --disable traefik" sh /tmp/k3s-install.sh`

	// Pass "sh", "-c", and the full command string as separate arguments to multipass exec
	cmd := exec.Command("multipass", "exec", vm, "--", "sh", "-c", downloadCmd+" && "+installCmd)

	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install k3s on %q: %w", vm, err)
	}

	return nil
}

func verifyK3sExists(vm string) (bool, error) {
	_, err := runMultipass("exec", vm, "--", "which", "k3s")
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("checking for k3s on VM %q: %w", vm, err)
}

func instanceExists(data []byte, vm string) (bool, error) {
	_, found, err := findInstance(data, vm)
	return found, err
}

// findInstance looks up the named instance in `multipass list --format json` output.
func findInstance(data []byte, vm string) (MultipassInstance, bool, error) {
	var list MultipassList
	if err := json.Unmarshal(data, &list); err != nil {
		return MultipassInstance{}, false, fmt.Errorf("parsing multipass list output: %w", err)
	}
	for _, instance := range list.Instances {
		if instance.Name == vm {
			return instance, true, nil
		}
	}
	return MultipassInstance{}, false, nil
}

func fetchKubeconfig(vm string) ([]byte, error) {
	output, err := runMultipass("exec", vm, "--", "sudo", "cat", "/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading kubeconfig from VM %q: %w", vm, err)
	}
	return output, nil
}

func nodeReady(vm string) (bool, error) {
	output, err := runMultipass("exec", vm, "--", "sudo", "k3s", "kubectl", "get", "nodes", "-o", `jsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}`)
	if err != nil {
		return false, fmt.Errorf("checking node readiness on %q: %w", vm, err)
	}
	if strings.TrimSpace(string(output)) == "True" {
		return true, nil
	}
	return false, nil
}

func nodeReadyCheck(vm string) error {
	if _, err := runMultipass("exec", vm, "--", "sudo", "k3s", "kubectl", "wait", "--for=condition=Ready", "node", "--all", "--timeout=10s"); err != nil {
		return fmt.Errorf("waiting for k3s node on VM %q to be ready: %w", vm, err)
	}
	return nil
}

func waitForNodeReady(vm string) error {
	attempts := 10
	var err error
	for range attempts {
		err = nodeReadyCheck(vm)
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("node on VM %q not ready after %d attempts: %w", vm, attempts, err)
}

func vmInfo(vm string) ([]byte, error) {
	output, err := runMultipass("info", vm, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("command 'multipass info' failed: %w", err)
	}
	return output, nil
}

func parseVMIP(data []byte, name string) (string, error) {
	var info MultipassInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("parsing multipass info output: %w", err)
	}
	vm, ok := info.Info[name]
	if !ok {
		return "", fmt.Errorf("VM %q not found in multipass info output", name)
	}
	if len(vm.IPv4) == 0 {
		return "", fmt.Errorf("VM %q has no IPv4 address", name)
	}
	return vm.IPv4[0], nil
}

func setKubeconfigServer(data []byte, ip string) []byte {
	return bytes.ReplaceAll(data, []byte("127.0.0.1"), []byte(ip))
}

type MultipassInfo struct {
	Info map[string]MultipassVMInfo `json:"info"`
}

type MultipassVMInfo struct {
	IPv4  []string `json:"ipv4"`
	State string   `json:"state"`
}

type MultipassList struct {
	Instances []MultipassInstance `json:"list"`
}

type MultipassInstance struct {
	Name  string   `json:"name"`
	State string   `json:"state"`
	IPv4  []string `json:"ipv4"`
}
