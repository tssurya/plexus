package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

func (r *ANDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	and := &v1beta1.AdministrativeNetworkDomain{}
	if err := r.Get(ctx, req.NamespacedName, and); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion via finalizer.
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

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(and, finalizerName) {
		controllerutil.AddFinalizer(and, finalizerName)
		if err := r.Update(ctx, and); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: set Ready=False with Reason=NoSubnets when len(and.Spec.Subnets) == 0.
	// An AND with no subnets is valid; this supports workflows where the
	// domain is created by one team (e.g. platform) and subnets are added
	// later by another (e.g. networking).

	// Reconcile via the backend.
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

	meta.SetStatusCondition(&and.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            fmt.Sprintf("All %d subnets reconciled by %s backend", len(and.Spec.Subnets), r.Backend.Name()),
		ObservedGeneration: and.Generation,
	})
	if err := r.Status().Update(ctx, and); err != nil {
		return ctrl.Result{}, err
	}

	if result.Requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ANDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.AdministrativeNetworkDomain{}).
		Named("and").
		Complete(r)
}
