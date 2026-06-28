package ovnkubernetes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	configv1beta1 "github.com/ovn-kubernetes/plexus/api/plexuscontrollerconfig/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
)

// OVNKubernetesBackend translates AND intent into OVN-Kubernetes resources
// (namespaces, CUDNs, RouteAdvertisements, VTEPs, etc.).
type OVNKubernetesBackend struct {
	client       client.Client
	log          logr.Logger
	vniAllocator *VNIAllocator
	config       *configv1beta1.OVNKubernetesConfig
}

func New(c client.Client, log logr.Logger, config *configv1beta1.OVNKubernetesConfig) *OVNKubernetesBackend {
	return &OVNKubernetesBackend{
		client:       c,
		log:          log.WithName("ovn-kubernetes-backend"),
		vniAllocator: NewVNIAllocator(),
		config:       config,
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

	if err := b.garbageCollectSubnets(ctx, and); err != nil {
		return backend.Result{}, fmt.Errorf("garbage collecting subnets: %w", err)
	}

	if err := b.reconcileVTEP(ctx); err != nil {
		return backend.Result{}, fmt.Errorf("reconciling VTEP: %w", err)
	}

	if err := b.reconcileRouteAdvertisements(ctx, and); err != nil {
		return backend.Result{}, fmt.Errorf("reconciling RouteAdvertisements: %w", err)
	}

	return b.checkResourceStatus(ctx, and)
}

func (b *OVNKubernetesBackend) Delete(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) error {
	b.log.Info("deleting AND resources", "name", and.Name)

	namespaces, err := b.listSubnetNamespaces(ctx, and)
	if err != nil {
		return fmt.Errorf("listing namespaces for deletion: %w", err)
	}
	for _, ns := range namespaces {
		subnetName := ns.Labels[labelSubnet]
		if err := b.deleteSubnet(ctx, and, &v1beta1.Subnet{Name: subnetName}); err != nil {
			return fmt.Errorf("deleting subnet %q: %w", subnetName, err)
		}
	}

	if err := b.deleteRouteAdvertisements(ctx, and); err != nil {
		return fmt.Errorf("deleting RouteAdvertisements: %w", err)
	}

	if err := b.deleteVTEP(ctx, and.Name); err != nil {
		return fmt.Errorf("deleting VTEP: %w", err)
	}

	return nil
}

func (b *OVNKubernetesBackend) listSubnetNamespaces(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) ([]corev1.Namespace, error) {
	var nsList corev1.NamespaceList
	if err := b.client.List(ctx, &nsList, client.MatchingLabels{labelNetworkDomain: and.Name}); err != nil {
		return nil, fmt.Errorf("listing namespaces for AND %q: %w", and.Name, err)
	}
	return nsList.Items, nil
}

func (b *OVNKubernetesBackend) garbageCollectSubnets(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) error {
	namespaces, err := b.listSubnetNamespaces(ctx, and)
	if err != nil {
		return err
	}

	desired := make(map[string]struct{}, len(and.Spec.Subnets))
	for _, s := range and.Spec.Subnets {
		desired[s.Name] = struct{}{}
	}

	for _, ns := range namespaces {
		subnetName := ns.Labels[labelSubnet]
		if _, ok := desired[subnetName]; ok {
			continue
		}
		b.log.Info("garbage collecting orphaned subnet", "subnet", subnetName, "namespace", ns.Name)
		if err := b.deleteSubnet(ctx, and, &v1beta1.Subnet{Name: subnetName}); err != nil {
			return fmt.Errorf("garbage collecting subnet %q: %w", subnetName, err)
		}
	}

	return nil
}
