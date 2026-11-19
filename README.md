# homelab-builder

`homelab` is a CLI for provisioning reproducible Kubernetes homelabs.

Today it creates a single-node [k3s](https://k3s.io) cluster in a local
[Multipass](https://multipass.run) VM and gives you a ready-to-use kubeconfig.
Multi-node clusters and AWS and GCP providers are coming soon.

## Requirements

- macOS or Linux (amd64 or arm64)
- [Multipass](https://multipass.run/install) installed and running
- `kubectl` (optional, to talk to the cluster)

## Install

Download the archive for your platform from the
[Releases](https://github.com/martinhWW/homelab-builder/releases) page, then:

```sh
tar -xzf homelab-builder_<version>_<os>_<arch>.tar.gz
sudo mv homelab /usr/local/bin/
homelab version
```

Each release includes a checksums file. To verify your download:

```sh
shasum -a 256 -c homelab-builder_<version>_checksums.txt --ignore-missing
```

On macOS, the binary isn't signed, so Gatekeeper may block it. Remove the
quarantine flag:

```sh
xattr -d com.apple.quarantine /usr/local/bin/homelab
```

### Build from source

Requires Go (see `go.mod` for the version).

```sh
git clone https://github.com/martinhWW/homelab-builder.git
cd homelab-builder
go build -o homelab .
```

Source builds report their version as `dev`.

## Usage

### Create a homelab

```sh
homelab create
```

This launches an Ubuntu 24.04 VM (2 CPUs, 4 GB memory, 30 GB disk), installs
k3s (with Traefik disabled), waits for the node to become ready, and writes a
kubeconfig to your machine. Connect with:

```sh
export KUBECONFIG="$HOME/.homelab/homelab/kubeconfig"
kubectl get nodes
```

Use `--name` to create a homelab with a different name (default `homelab`).
The name is also the Multipass VM name.

### Check status

```sh
homelab status
```

Shows the VM state and whether the k3s node is ready.

### List homelabs

```sh
homelab list
```

Lists every homelab that `homelab` created, with its provider, VM state, and
node count:

```
NAME     PROVIDER  STATE      NODES
dev      local     Running    1
staging  local     Stopped    1
```

A homelab whose VM was deleted outside of `homelab` shows `Not found`. Other
Multipass VMs are never listed. `list` is read-only.

### Destroy a homelab

```sh
homelab destroy
```

Deletes the VM and removes the homelab's local files, including the
kubeconfig. Asks for confirmation unless you pass `--force`.

## How it works

Each homelab has a directory at `~/.homelab/<name>/` containing:

- `state.json`: records that `homelab` created this VM
- `kubeconfig`: credentials for the cluster (treat as sensitive)

`create` and `destroy` refuse to touch a Multipass VM that has no state file,
so `homelab` never modifies VMs it didn't create. `list` finds homelabs by
their state files for the same reason.
