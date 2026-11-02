// Package testutil provides reusable test infrastructure for all test packages.
// This package avoids import cycles by only exposing helpers that don't depend on
// API types. API-specific fixture builders (makeAND, makeSubnet, etc.) live in their
// respective test files to prevent circular imports.
//
// Pattern for test helpers:
// 1. Truly reusable infrastructure (envtest setup, module root finding) → testutil
// 2. API-specific fixtures (AND, Subnet, Config builders) → local test files
// 3. Domain-specific helpers (marking resources accepted) → local test files
//
// This pattern prevents OVN-Kubernetes-style helper duplication while avoiding
// import cycles that would arise from putting API types in testutil.
package testutil

import (
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// FindModuleRoot locates the project root by searching for go.mod in the current directory and parent directories.
func FindModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProjectCRDPaths returns the directory holding the project's own generated
// CRDs, resolved against the module root so it works from any test binary's
// working directory. Suites that only exercise Plexus types want this.
func ProjectCRDPaths() []string {
	return []string{filepath.Join(FindModuleRoot(), "config", "crd")}
}

// CRDPaths returns the project CRDs plus the committed OVN-Kubernetes CRD
// fixtures. Suites that exercise resources the backend creates (CUDN,
// RouteAdvertisements, VTEP) want this.
func CRDPaths() []string {
	root := FindModuleRoot()
	return []string{
		filepath.Join(root, "config", "crd"),
		filepath.Join(root, "test", "testdata", "crds"),
	}
}

// EnvtestHarness encapsulates envtest setup and teardown.
// Env is exposed so suites that need the *rest.Config (e.g. to build a
// controller-runtime Manager) can reach it via Env.Config.
type EnvtestHarness struct {
	Env    *envtest.Environment
	Client client.Client
	Scheme *runtime.Scheme
}

// StartEnvtest starts envtest with the given CRD paths and scheme registration
// function. Pass nil crdPaths for suites that need only the built-in API types.
// Caller is responsible for calling Stop() in AfterSuite.
func StartEnvtest(crdPaths []string, registerScheme func(*runtime.Scheme) error) (*EnvtestHarness, error) {
	scheme := runtime.NewScheme()
	if err := registerScheme(scheme); err != nil {
		return nil, err
	}

	env := &envtest.Environment{
		CRDDirectoryPaths: crdPaths,
		// Fail loudly on a mistyped CRD path rather than silently starting an
		// apiserver with the CRDs missing, which surfaces as confusing
		// "no matches for kind" errors deep inside individual specs.
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := env.Start()
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return &EnvtestHarness{
		Env:    env,
		Client: c,
		Scheme: scheme,
	}, nil
}

// Stop tears down the envtest environment
func (h *EnvtestHarness) Stop() error {
	if h.Env != nil {
		return h.Env.Stop()
	}
	return nil
}
