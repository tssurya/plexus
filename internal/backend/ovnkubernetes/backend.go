package ovnkubernetes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
)

// OVNKubernetesBackend translates AND intent into OVN-Kubernetes resources
// (namespaces, UDNs, CNCs, RouteAdvertisements, etc.).
type OVNKubernetesBackend struct {
	client client.Client
	log    logr.Logger
}

func New(c client.Client, log logr.Logger) *OVNKubernetesBackend {
	return &OVNKubernetesBackend{
		client: c,
		log:    log.WithName("ovn-kubernetes-backend"),
	}
}

func (b *OVNKubernetesBackend) Name() string {
	return "ovn-kubernetes"
}

func (b *OVNKubernetesBackend) Reconcile(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) (backend.Result, error) {
	b.log.Info("reconciling AND", "name", and.Name, "subnets", len(and.Spec.Subnets))

	for _, subnet := range and.Spec.Subnets {
		if err := b.reconcileSubnet(ctx, and, &subnet); err != nil {
			return backend.Result{}, fmt.Errorf("reconciling subnet %q: %w", subnet.Name, err)
		}
	}

	return backend.Result{}, nil
}

func (b *OVNKubernetesBackend) Delete(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) error {
	b.log.Info("deleting AND resources", "name", and.Name)

	for _, subnet := range and.Spec.Subnets {
		if err := b.deleteSubnet(ctx, and, &subnet); err != nil {
			return fmt.Errorf("deleting subnet %q: %w", subnet.Name, err)
		}
	}

	return nil
}
