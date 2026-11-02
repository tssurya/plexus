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

## Unit Testing

Framework: **Ginkgo v2 + Gomega**. Ginkgo is a library on top of `go test`,
not a replacement — `make test`, `make check`, and `make unit` all run
`go test ./... -race`. It is used for consistency with
ovn-kubernetes/controller-runtime and for suite-level setup/teardown, async
assertions, and table-driven specs. Ginkgo overhead is negligible; envtest
startup and `-race` dominate the unit-test lane.

**Layout**
- One `suite_test.go` per package with `func TestX(t *testing.T)` calling
  `RunSpecs`.
- Specs live in `*_test.go` next to the code under test.
- API type tests use an external `package v1beta1_test` so they only
  exercise the exported API.
- Fixtures are named `testX` and stay local to the package. Shared envtest
  infrastructure lives in `internal/testutil`.
- CLI fake-client helpers live in `pkg/cli/cli_test.go` and stay there; they
  depend on `cliScheme` and `ClientFactory`. Do not move them into
  `internal/testutil` — generic test infra must not depend on `pkg/cli` or
  the API types (import cycle).

**Test layers**
- Pure logic (no envtest): VNI allocator, CIDR/label helpers, CLI
  validation.
- CLI/controller logic: controller-runtime fake client. CLI commands take
  a `ClientFactory`; tests inject a fake.
- API schema/admission/reconciler behavior: envtest.
- Full cluster/CNI: kind/e2e, not unit tests.

**Envtest**
- `make test`/`make check` runs `setup-envtest` once and sets
  `KUBEBUILDER_ASSETS`; binaries are cached in `bin/k8s`.
- Each envtest-enabled package starts/stops its own kube-apiserver+etcd in
  `BeforeSuite`/`AfterSuite`. This is deliberate: Go test packages are
  separate test binaries that can run in parallel, and separate control
  planes give isolation. Within a job, `go test -p` (default `GOMAXPROCS`)
  caps concurrency.
- Use `internal/testutil.StartEnvtest` with `ProjectCRDPaths()` for
  Plexus-only suites and `CRDPaths()` when OVN-K CRDs are needed. Use
  `internal/testscheme.AddAll` for the full scheme.
- `internal/testutil` is module-wide but not importable outside the module.
  Keep it generic and free of API types so in-package API tests can import
  it without an import cycle.
- Only use envtest when the test needs real API behavior: CRD
  schema/CEL/defaulting, admission, status subresources, or watches.
  Otherwise use the fake client or pure tests.
- Envtest has no kube-controller-manager: namespaces do not finalize,
  there is no owner-reference GC, scheduler, kubelet, or CNI. Use unique
  names per spec and do not wait for namespace deletion.
- Keep envtest suites few and grouped by API surface. If the count grows,
  consolidate or shard packages across CI jobs; reduce `go test -p` only
  as a memory-stability fallback.

**Assertions and CLI**
- Use `Eventually`/`Consistently` with explicit timeout/interval for
  asynchronous reconciler behavior, and `GinkgoRecover` in goroutines
  (e.g. `manager.Start`).
- Do not assert on production constants for externally visible strings;
  use literals or test-local constants.
- Verify actual object state with the fake client, not just command output.

**Code generation and test files**
- `make generate` uses `bin/controller-gen`, installed from the `tools`
  module; the pinned version lives in `tools/go.mod`.
- controller-gen loads normal packages, not test variants, so `_test.go`
  files are not included even when they live next to API types or
  controllers. No special handling is needed; `make check`/`go test ./...`
  still runs them. Keeping tests beside the API types they exercise is
  common (e.g. Kubernetes API `core/v1/types_test.go`).

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
