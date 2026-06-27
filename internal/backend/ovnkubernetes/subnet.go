package ovnkubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

const (
	labelNetworkDomain = "plexus.io/network-domain"
	labelSubnet        = "plexus.io/subnet"
	labelSubnetType    = "plexus.io/subnet-type"
)

// namespaceName returns the deterministic namespace name for a subnet:
// "<and-name>-<subnet-name>".
func namespaceName(and *v1beta1.AdministrativeNetworkDomain, subnet *v1beta1.Subnet) string {
	return fmt.Sprintf("%s-%s", and.Name, subnet.Name)
}

// reconcileSubnet ensures the namespace for the given subnet exists with
// the correct labels. UDN creation will be added once the OVN-Kubernetes
// API types are wired in.
func (b *OVNKubernetesBackend) reconcileSubnet(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain, subnet *v1beta1.Subnet) error {
	nsName := namespaceName(and, subnet)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				labelNetworkDomain: and.Name,
				labelSubnet:        subnet.Name,
				labelSubnetType:    string(subnet.Type),
			},
		},
	}

	existing := &corev1.Namespace{}
	err := b.client.Get(ctx, client.ObjectKeyFromObject(ns), existing)
	if apierrors.IsNotFound(err) {
		b.log.Info("creating namespace for subnet", "namespace", nsName, "subnet", subnet.Name)
		return b.client.Create(ctx, ns)
	}
	if err != nil {
		return fmt.Errorf("getting namespace %q: %w", nsName, err)
	}

	// TODO: reconcile labels if they've drifted
	// TODO: create UDN (L2 EVPN) in this namespace
	// TODO: create RouteAdvertisements for Public subnets
	// TODO: create CNC for intra-domain routing between non-Isolated subnets

	return nil
}

// deleteSubnet removes the namespace (and all resources within it) for the
// given subnet.
func (b *OVNKubernetesBackend) deleteSubnet(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain, subnet *v1beta1.Subnet) error {
	nsName := namespaceName(and, subnet)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	err := b.client.Delete(ctx, ns)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
