package ovnkubernetes

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rav1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/routeadvertisements/v1"
	udnv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	vtepv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
)

// checkResourceStatus inspects the status conditions of all child resources
// (VTEP, CUDNs, RouteAdvertisements) and returns a Result indicating whether
// the AND should be marked as not-ready.
func (b *OVNKubernetesBackend) checkResourceStatus(ctx context.Context, and *v1beta1.AdministrativeNetworkDomain) (backend.Result, error) {
	vtep := &vtepv1.VTEP{}
	if err := b.client.Get(ctx, client.ObjectKey{Name: vtepName}, vtep); err != nil {
		return backend.Result{}, fmt.Errorf("getting VTEP %q status: %w", vtepName, err)
	}
	cond := apimeta.FindStatusCondition(vtep.Status.Conditions, "Accepted")
	if cond == nil || cond.Status != metav1.ConditionTrue {
		msg := "VTEP not yet accepted"
		if cond != nil {
			msg = cond.Message
		}
		return backend.Result{
			Requeue:       true,
			StatusReason:  "VTEPNotReady",
			StatusMessage: msg,
		}, nil
	}

	for i := range and.Spec.Subnets {
		name := cudnName(and, &and.Spec.Subnets[i])
		cudn := &udnv1.ClusterUserDefinedNetwork{}
		if err := b.client.Get(ctx, client.ObjectKey{Name: name}, cudn); err != nil {
			return backend.Result{}, fmt.Errorf("getting CUDN %q status: %w", name, err)
		}
		cond := apimeta.FindStatusCondition(cudn.Status.Conditions, "NetworkCreated")
		if cond == nil || cond.Status != metav1.ConditionTrue {
			msg := "network not yet created"
			if cond != nil {
				msg = cond.Message
			}
			return backend.Result{
				Requeue:       true,
				StatusReason:  "SubnetsNotReady",
				StatusMessage: fmt.Sprintf("CUDN %q not ready: %s", name, msg),
			}, nil
		}
	}

	hasPublic := false
	for i := range and.Spec.Subnets {
		if and.Spec.Subnets[i].Type == v1beta1.SubnetTypePublic {
			hasPublic = true
			break
		}
	}
	if hasPublic {
		name := raName(and)
		ra := &rav1.RouteAdvertisements{}
		if err := b.client.Get(ctx, client.ObjectKey{Name: name}, ra); err != nil {
			return backend.Result{}, fmt.Errorf("getting RouteAdvertisements %q status: %w", name, err)
		}
		cond := apimeta.FindStatusCondition(ra.Status.Conditions, "Accepted")
		if cond == nil || cond.Status != metav1.ConditionTrue {
			msg := "RouteAdvertisements not yet accepted"
			if cond != nil {
				msg = cond.Message
			}
			return backend.Result{
				Requeue:       true,
				StatusReason:  "SubnetsNotReady",
				StatusMessage: fmt.Sprintf("RouteAdvertisements %q not accepted: %s", name, msg),
			}, nil
		}
	}

	return backend.Result{}, nil
}
