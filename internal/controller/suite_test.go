package controller_test

import (
	"context"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	andv1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
	"github.com/ovn-kubernetes/plexus/internal/controller"
	"github.com/ovn-kubernetes/plexus/internal/multicluster"
	"github.com/ovn-kubernetes/plexus/internal/testscheme"
	"github.com/ovn-kubernetes/plexus/internal/testutil"
)

var (
	k8sClient client.Client
	harness   *testutil.EnvtestHarness
	ctx       context.Context
	cancel    context.CancelFunc
	fakeBack  *fakeBackend
)

// fakeBackend is a test double for backend.Backend.
// All fields are mutex-protected so the reconciler goroutine and test
// goroutine can access them concurrently without a race.
type fakeBackend struct {
	mu              sync.Mutex
	reconcileResult backend.Result
	reconcileErr    error
	deleteErr       error
	deleteCalled    chan struct{}
	reconcileCount  int
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{deleteCalled: make(chan struct{}, 1)}
}

func (f *fakeBackend) setReconcile(result backend.Result, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconcileResult = result
	f.reconcileErr = err
}

// reset clears all state between tests.
func (f *fakeBackend) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconcileResult = backend.Result{}
	f.reconcileErr = nil
	f.deleteErr = nil
	f.reconcileCount = 0
	// drain channel
	select {
	case <-f.deleteCalled:
	default:
	}
}

func (f *fakeBackend) getReconcileCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reconcileCount
}

func (f *fakeBackend) setDeleteErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteErr = err
}

func (f *fakeBackend) Reconcile(_ context.Context, _ *andv1beta1.AdministrativeNetworkDomain) (backend.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconcileCount++
	return f.reconcileResult, f.reconcileErr
}

func (f *fakeBackend) Delete(_ context.Context, _ *andv1beta1.AdministrativeNetworkDomain) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case f.deleteCalled <- struct{}{}:
	default:
	}
	return f.deleteErr
}

func (f *fakeBackend) Name() string { return "fake" }

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	// envtest binaries are downloaded via 'make test'.
	// Skip gracefully when they are not available so that
	// 'go test ./...' works for contributors without the binaries installed.
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		Skip("KUBEBUILDER_ASSETS not set — run 'make test' to download envtest binaries first")
	}

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	var err error
	harness, err = testutil.StartEnvtest(testutil.CRDPaths(), testscheme.AddAll)
	Expect(err).NotTo(HaveOccurred())

	k8sClient = harness.Client
	fakeBack = newFakeBackend()

	inventory := multicluster.NewSecretInventory(multicluster.SecretInventoryOptions{
		HubClient: k8sClient,
		HubLabels: map[string]string{},
		Scheme:    harness.Scheme,
		Log:       ctrl.Log,
	})

	cfg := harness.Env.Config
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: harness.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // disable metrics server in tests
		},
		HealthProbeBindAddress: "0", // disable health probes in tests
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&controller.ANDReconciler{
		Client:    mgr.GetClient(),
		Backend:   fakeBack,
		Inventory: inventory,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
	if harness != nil {
		Expect(harness.Stop()).To(Succeed())
	}
})
