package v1beta1

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ovn-kubernetes/plexus/internal/testutil"
)

var (
	k8sClient client.Client
	harness   *testutil.EnvtestHarness
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestPlexusControllerConfigAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PlexusControllerConfig API Suite")
}

var _ = BeforeSuite(func() {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		Skip("KUBEBUILDER_ASSETS not set — run 'make test' to include envtest")
	}

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	var err error
	harness, err = testutil.StartEnvtest(testutil.ProjectCRDPaths(), AddToScheme)
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

// Test constants for OVN-Kubernetes config values
const (
	testVTEPCIDR     = "172.18.0.0/16"
	testVTEPCIDRIPv6 = "fd00::/64"
	testVTEPCIDRBad  = "172.18.0.1/16" // host bits set
)

// testConfig creates a PlexusControllerConfig with OVN-Kubernetes backend.
func testConfig(name string, vtepCIDRs ...string) *PlexusControllerConfig {
	cidrs := make([]CIDR, len(vtepCIDRs))
	for i, c := range vtepCIDRs {
		cidrs[i] = CIDR(c)
	}
	return &PlexusControllerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: PlexusControllerConfigSpec{
			Backend: "ovn-kubernetes",
			OVNKubernetes: OVNKubernetesConfig{
				VTEPCIDRs: cidrs,
				FRRConfigurationSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "frr"},
				},
			},
		},
	}
}

var _ = Describe("PlexusControllerConfig", func() {
	var created *PlexusControllerConfig

	AfterEach(func() {
		if created != nil {
			_ = k8sClient.Delete(ctx, created)
			created = nil
		}
	})

	It("accepts a valid ovn-kubernetes configuration", func() {
		created = &PlexusControllerConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "plexus"},
			Spec: PlexusControllerConfigSpec{
				Backend: "ovn-kubernetes",
				OVNKubernetes: OVNKubernetesConfig{
					VTEPCIDRs: []CIDR{"172.18.0.0/16"},
					FRRConfigurationSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "frr"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())
	})

	It("rejects ovn-kubernetes backend without ovnKubernetes configuration", func() {
		cfg := &PlexusControllerConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "plexus-missing-ovnk"},
			Spec: PlexusControllerConfigSpec{
				Backend: "ovn-kubernetes",
			},
		}
		err := k8sClient.Create(ctx, cfg)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("rejects an invalid VTEP CIDR", func() {
		cfg := &PlexusControllerConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "plexus-bad-cidr"},
			Spec: PlexusControllerConfigSpec{
				Backend: "ovn-kubernetes",
				OVNKubernetes: OVNKubernetesConfig{
					VTEPCIDRs: []CIDR{"not-a-cidr"},
					FRRConfigurationSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "frr"},
					},
				},
			},
		}
		err := k8sClient.Create(ctx, cfg)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	Describe("DeepCopy", func() {
		It("deep copies PlexusControllerConfig without sharing references", func() {
			original := testConfig("plexus", testVTEPCIDR, testVTEPCIDRIPv6)
			copy := original.DeepCopy()

			Expect(copy).To(Equal(original), "copy should equal original")
			Expect(copy).NotTo(BeIdenticalTo(original), "copy should be a different object")

			// Mutate copy - original must remain unchanged
			copy.Spec.OVNKubernetes.VTEPCIDRs[0] = "192.168.0.0/16"
			copy.Spec.OVNKubernetes.FRRConfigurationSelector.MatchLabels["app"] = "modified"
			Expect(original.Spec.OVNKubernetes.VTEPCIDRs[0]).To(Equal(CIDR(testVTEPCIDR)))
			Expect(original.Spec.OVNKubernetes.FRRConfigurationSelector.MatchLabels["app"]).To(Equal("frr"))
		})

		It("deep copies OVNKubernetes config correctly", func() {
			original := &PlexusControllerConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "plexus-no-ovnk"},
				Spec: PlexusControllerConfigSpec{
					Backend: "future-backend",
					OVNKubernetes: OVNKubernetesConfig{
						VTEPCIDRs: []CIDR{CIDR(testVTEPCIDR)},
					},
				},
			}
			copy := original.DeepCopy()
			Expect(copy.Spec.OVNKubernetes.VTEPCIDRs[0]).To(Equal(CIDR(testVTEPCIDR)))
		})

		It("deep copies with DeepCopyInto", func() {
			original := testConfig("plexus", testVTEPCIDR)
			var target PlexusControllerConfig
			original.DeepCopyInto(&target)

			Expect(&target).To(Equal(original), "target should equal original")
			target.Spec.OVNKubernetes.VTEPCIDRs[0] = "192.168.0.0/16"
			Expect(original.Spec.OVNKubernetes.VTEPCIDRs[0]).To(Equal(CIDR(testVTEPCIDR)), "original should not change")
		})

		It("deep copies with DeepCopyObject", func() {
			original := testConfig("plexus", testVTEPCIDR)
			copyObj := original.DeepCopyObject()

			copyConfig, ok := copyObj.(*PlexusControllerConfig)
			Expect(ok).To(BeTrue(), "DeepCopyObject should return *PlexusControllerConfig")
			Expect(copyConfig).To(Equal(original))
		})

		It("deep copies OVNKubernetesConfig", func() {
			original := &OVNKubernetesConfig{
				VTEPCIDRs: []CIDR{CIDR(testVTEPCIDR)},
				FRRConfigurationSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "frr", "zone": "a"},
				},
			}
			copy := original.DeepCopy()

			Expect(copy).To(Equal(original), "copy should equal original")
			copy.FRRConfigurationSelector.MatchLabels["zone"] = "b"
			Expect(original.FRRConfigurationSelector.MatchLabels["zone"]).To(Equal("a"), "original should not change")
		})

		It("deep copies PlexusControllerConfigList", func() {
			original := &PlexusControllerConfigList{
				Items: []PlexusControllerConfig{
					*testConfig("config1", testVTEPCIDR),
					*testConfig("config2", testVTEPCIDR),
				},
			}
			copy := original.DeepCopy()

			Expect(copy).To(Equal(original), "copy should equal original list")
			copy.Items[0].Name = "modified"
			Expect(original.Items[0].Name).To(Equal("config1"), "original list item should not change")
		})
	})

	Describe("Additional Validation Edge Cases", func() {
		It("rejects VTEP CIDR with host bits set", func() {
			cfg := testConfig("plexus-host-bits", testVTEPCIDRBad)
			err := k8sClient.Create(ctx, cfg)
			Expect(err).To(HaveOccurred(), "creating config with host bits in CIDR should fail")
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("accepts multiple VTEP CIDRs (dual-stack)", func() {
			created = testConfig("plexus-dual-stack", testVTEPCIDR, testVTEPCIDRIPv6)
			Expect(k8sClient.Create(ctx, created)).To(Succeed(), "creating dual-stack VTEP config should succeed")
		})

		It("accepts FRRConfigurationSelector with MatchExpressions", func() {
			created = testConfig("plexus-match-expr", testVTEPCIDR)
			created.Spec.OVNKubernetes.FRRConfigurationSelector = metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"frr", "bgp"}},
				},
			}
			Expect(k8sClient.Create(ctx, created)).To(Succeed(), "creating config with match expressions should succeed")
		})

		It("rejects empty FRRConfigurationSelector", func() {
			created = testConfig("plexus-empty-selector", testVTEPCIDR)
			created.Spec.OVNKubernetes.FRRConfigurationSelector = metav1.LabelSelector{}
			err := k8sClient.Create(ctx, created)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("matchLabel or matchExpression"))
		})
	})
})
