# AGENTS.md — Plexus

Project context for AI coding agents.

## Project Overview

Plexus is a network orchestrator that introduces the
**AdministrativeNetworkDomain** (AND) CRD — a VPC-like abstraction for
Kubernetes. It translates high-level network intent (subnets, routing,
security, gateways) into backend-specific resources. OVN-Kubernetes is the
reference backend.

## Repository Layout

```text
api/               # CRD API types, one directory per CRD kind
  administrativenetworkdomain/v1beta1/  # AND CRD (plexus.io/v1beta1)
cmd/controller/    # Plexus controller entrypoint
cmd/kubectl-plexus/# CLI plugin entrypoint
internal/          # Private implementation (controller, backend, IPAM)
pkg/cli/           # CLI command implementations
config/            # CRD manifests, RBAC, deployment YAML, samples
hack/              # Code generation scripts
test/              # E2E and integration tests
docs/              # Documentation
ui/                # Web UI plugin (React + PatternFly) — future
```

## Build and Test

```bash
make build      # Build controller and CLI binaries
make test       # Run unit tests
make lint       # Format, vet, and lint
make generate   # Run deepcopy code generator
make manifests  # Generate CRD YAML from Go types
make install    # Install CRDs into the configured cluster
make run        # Run controller locally
```

## Key Conventions

- API group: `plexus.io/v1beta1`
- CRD short name: `and`
- Backend interface in `internal/backend/interface.go`
- Controller uses `controller-runtime` reconciler pattern
- CLI uses `cobra` and is usable as `kubectl plexus` or standalone `plexus`
- Apache 2.0 license
- Follows ovn-kubernetes commit message conventions

## Architecture

```
User (CLI / UI / YAML)
  → AdministrativeNetworkDomain CRD (Kubernetes API)
    → Plexus Controller (reconciler)
      → Backend Plugin Interface
        → OVN-Kubernetes Backend
          → Namespaces, UDNs, CNCs, RouteAdvertisements, NetworkPolicy
```

The controller watches AND CRs and delegates to the configured backend.
The OVN-Kubernetes backend creates namespaces + UDNs (L2 EVPN) per subnet,
RouteAdvertisements for Public subnets, and intra-domain routing via CNC.
