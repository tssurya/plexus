package ovnkubernetes

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ovn-kubernetes/plexus/internal/testscheme"
	"github.com/ovn-kubernetes/plexus/internal/testutil"
)

var (
	k8sClient client.Client
	harness   *testutil.EnvtestHarness
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestOVNKubernetes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OVNKubernetes Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	// VNIAllocator specs are pure unit tests and always run. Resource
	// specs that talk to the API server only run when envtest binaries
	// are present (make test).
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		return
	}

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var err error
	harness, err = testutil.StartEnvtest(testutil.CRDPaths(), testscheme.AddAll)
	Expect(err).NotTo(HaveOccurred())

	k8sClient = harness.Client
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
	if harness != nil {
		Expect(harness.Stop()).To(Succeed())
	}
})
