// Package testscheme builds the runtime scheme shared by test suites that
// exercise Plexus resources against envtest.
//
// This lives outside internal/testutil deliberately: internal/testutil is
// generic envtest infrastructure and must not import the API types, so API
// suites can import it without an import cycle. testscheme is the opposite:
// it imports the API types on purpose and is used by suites that reconcile
// real resources (backend, controller). API suites pass their own
// AddToScheme directly instead.
package testscheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	rav1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/routeadvertisements/v1"
	udnv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	vtepv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"

	andv1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	configv1beta1 "github.com/ovn-kubernetes/plexus/api/plexuscontrollerconfig/v1beta1"
)

// AddAll registers the built-in Kubernetes types, both Plexus API groups and
// the OVN-Kubernetes types the backend creates. Suites reconciling real
// resources need the full set; keeping it in one place means adding a CRD is
// a single edit rather than one per suite.
func AddAll(s *runtime.Scheme) error {
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		andv1beta1.AddToScheme,
		configv1beta1.AddToScheme,
		udnv1.AddToScheme,
		rav1.AddToScheme,
		vtepv1.AddToScheme,
	} {
		if err := add(s); err != nil {
			return err
		}
	}
	return nil
}
