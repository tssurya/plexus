package ovnkubernetes

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vtepv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"

	andv1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

const labelManagedBy = "plexus.io/managed-by"

func (b *OVNKubernetesBackend) reconcileVTEP(ctx context.Context) error {
	cidrs := make([]vtepv1.CIDR, len(b.config.VTEPCIDRs))
	for i, c := range b.config.VTEPCIDRs {
		cidrs[i] = vtepv1.CIDR(c)
	}

	existing := &vtepv1.VTEP{}
	err := b.client.Get(ctx, client.ObjectKey{Name: vtepName}, existing)
	if apierrors.IsNotFound(err) {
		vtep := &vtepv1.VTEP{
			ObjectMeta: metav1.ObjectMeta{
				Name: vtepName,
				Labels: map[string]string{
					labelManagedBy: "plexus",
				},
			},
			Spec: vtepv1.VTEPSpec{
				CIDRs: cidrs,
				Mode:  vtepv1.VTEPModeUnmanaged,
			},
		}
		b.log.Info("creating shared VTEP", "name", vtepName)
		if err := b.client.Create(ctx, vtep); err != nil {
			return fmt.Errorf("creating VTEP %q: %w", vtepName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting VTEP %q: %w", vtepName, err)
	}

	if !cidrsEqual(existing.Spec.CIDRs, cidrs) {
		existing.Spec.CIDRs = cidrs
		b.log.Info("updating VTEP CIDRs", "name", vtepName)
		if err := b.client.Update(ctx, existing); err != nil {
			return fmt.Errorf("updating VTEP %q: %w", vtepName, err)
		}
	}

	return nil
}

func (b *OVNKubernetesBackend) deleteVTEP(ctx context.Context, deletingAND string) error {
	var andList andv1beta1.AdministrativeNetworkDomainList
	if err := b.client.List(ctx, &andList); err != nil {
		return fmt.Errorf("listing ANDs for VTEP ref count: %w", err)
	}

	remaining := 0
	for i := range andList.Items {
		if andList.Items[i].Name != deletingAND && andList.Items[i].DeletionTimestamp.IsZero() {
			remaining++
		}
	}
	if remaining > 0 {
		b.log.Info("skipping VTEP deletion, other ANDs still exist", "remaining", remaining)
		return nil
	}

	vtep := &vtepv1.VTEP{
		ObjectMeta: metav1.ObjectMeta{
			Name: vtepName,
		},
	}
	err := b.client.Delete(ctx, vtep)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting VTEP %q: %w", vtepName, err)
	}
	return nil
}

func cidrsEqual(a []vtepv1.CIDR, b []vtepv1.CIDR) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
