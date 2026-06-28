package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	udnv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
)

const finalizerName = "plexus.io/and-protection"

// ANDReconciler reconciles AdministrativeNetworkDomain resources.
type ANDReconciler struct {
	client.Client
	Backend backend.Backend
}

// +kubebuilder:rbac:groups=plexus.io,resources=administrativenetworkdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=plexus.io,resources=administrativenetworkdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=plexus.io,resources=administrativenetworkdomains/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.ovn.org,resources=clusteruserdefinednetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.ovn.org,resources=routeadvertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.ovn.org,resources=vteps,verbs=get;list;watch;create;update;patch;delete

func (r *ANDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	and := &v1beta1.AdministrativeNetworkDomain{}
	if err := r.Get(ctx, req.NamespacedName, and); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !and.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(and, finalizerName) {
			logger.Info("deleting AND resources", "name", and.Name)
			if err := r.Backend.Delete(ctx, and); err != nil {
				return ctrl.Result{}, fmt.Errorf("backend delete: %w", err)
			}
			controllerutil.RemoveFinalizer(and, finalizerName)
			if err := r.Update(ctx, and); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(and, finalizerName) {
		controllerutil.AddFinalizer(and, finalizerName)
		if err := r.Update(ctx, and); err != nil {
			return ctrl.Result{}, err
		}
	}

	noSubnets := len(and.Spec.Subnets) == 0

	result, err := r.Backend.Reconcile(ctx, and)
	if err != nil {
		meta.SetStatusCondition(&and.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ReconcileError",
			Message:            err.Error(),
			ObservedGeneration: and.Generation,
		})
		_ = r.Status().Update(ctx, and)
		return ctrl.Result{}, fmt.Errorf("backend reconcile: %w", err)
	}

	if noSubnets {
		meta.SetStatusCondition(&and.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "NoSubnets",
			Message:            "No subnets defined; domain is awaiting subnet configuration",
			ObservedGeneration: and.Generation,
		})
	} else {
		meta.SetStatusCondition(&and.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Reconciled",
			Message:            fmt.Sprintf("All %d subnets reconciled by %s backend", len(and.Spec.Subnets), r.Backend.Name()),
			ObservedGeneration: and.Generation,
		})
	}
	if err := r.Status().Update(ctx, and); err != nil {
		return ctrl.Result{}, err
	}

	if result.Requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func andNameFromLabels(_ context.Context, obj client.Object) []reconcile.Request {
	andName := obj.GetLabels()["plexus.io/network-domain"]
	if andName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: andName}}}
}

func (r *ANDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.AdministrativeNetworkDomain{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(andNameFromLabels)).
		Watches(&udnv1.ClusterUserDefinedNetwork{}, handler.EnqueueRequestsFromMapFunc(andNameFromLabels)).
		Named("and").
		Complete(r)
}
