package backend

import (
	"context"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

// Backend translates AdministrativeNetworkDomain intent into platform-specific
// resources. OVN-Kubernetes is the reference implementation; future backends
// (cloud providers, other CNIs) implement the same interface.
type Backend interface {
	// Reconcile ensures that the backend resources for the given AND match
	// the desired state. It is called on every reconciliation loop. The
	// implementation must be idempotent.
	Reconcile(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) (Result, error)

	// Delete removes all backend resources associated with the given AND.
	Delete(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) error

	// Name returns a human-readable identifier for this backend (e.g. "ovn-kubernetes").
	Name() string
}

// Result carries information back from a backend reconciliation pass.
type Result struct {
	// Requeue indicates that the controller should re-enqueue the AND
	// for another reconciliation pass (e.g. because a backend resource
	// is not yet ready).
	Requeue bool
}
