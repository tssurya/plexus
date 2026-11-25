package v1beta1_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/testutil"
)

var (
	k8sClient client.Client
	harness   *testutil.EnvtestHarness
	ctx       context.Context
	cancel    context.CancelFunc
)

const (
	timeout  = 10 * time.Second
	interval = 250 * time.Millisecond
)

func TestAdministrativeNetworkDomainAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AdministrativeNetworkDomain API Suite")
}

var _ = BeforeSuite(func() {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		Skip("KUBEBUILDER_ASSETS not set — run 'make test' to include envtest")
	}

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	var err error
	harness, err = testutil.StartEnvtest(testutil.ProjectCRDPaths(), v1beta1.AddToScheme)
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

// Test constants for common subnet and CIDR values
const (
	testSubnetWeb    = "web"
	testSubnetDB     = "db"
	testCIDRWeb      = "10.0.1.0/24"
	testCIDRDB       = "10.0.2.0/24"
	testCIDRModified = "192.168.1.0/24"
	testCIDRIPv6     = "fd00::/64"
)

// testAND creates an AdministrativeNetworkDomain with the given name and subnets.
// This reduces boilerplate in tests that need to create AND fixtures.
// Note: ResourceVersion is not set - it will be assigned by the API server on create.
func testAND(name string, subnets ...v1beta1.Subnet) *v1beta1.AdministrativeNetworkDomain {
	return &v1beta1.AdministrativeNetworkDomain{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1beta1.AdministrativeNetworkDomainSpec{Subnets: subnets},
	}
}

// testSubnet creates a Subnet with the given parameters.
func testSubnet(name, cidr string, subnetType v1beta1.SubnetType) v1beta1.Subnet {
	return v1beta1.Subnet{
		Name:  name,
		CIDRs: []v1beta1.CIDR{v1beta1.CIDR(cidr)},
		Type:  subnetType,
	}
}

var _ = Describe("AdministrativeNetworkDomain", func() {
	var created *v1beta1.AdministrativeNetworkDomain

	AfterEach(func() {
		if created != nil {
			_ = k8sClient.Delete(ctx, created)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(created), &v1beta1.AdministrativeNetworkDomain{})
			}, timeout, interval).Should(MatchError(ContainSubstring("not found")))
			created = nil
		}
	})

	It("accepts a valid AND with Public, Private, and Isolated subnets", func() {
		created = &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-valid"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{Name: "web", CIDRs: []v1beta1.CIDR{"10.0.1.0/24"}, Type: v1beta1.SubnetTypePublic},
					{Name: "app", CIDRs: []v1beta1.CIDR{"10.0.10.0/24"}, Type: v1beta1.SubnetTypePrivate},
					{Name: "db", CIDRs: []v1beta1.CIDR{"10.0.20.0/24"}, Type: v1beta1.SubnetTypeIsolated},
				},
			},
		}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())
	})

	It("rejects an AND with no subnets", func() {
		// Subnets is required with MinItems=1: at least one subnet is needed.
		created = &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-no-subnets"},
			Spec:       v1beta1.AdministrativeNetworkDomainSpec{},
		}
		err := k8sClient.Create(ctx, created)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("subnets"))
	})

	It("rejects an AND with an explicitly empty subnet list", func() {
		// MinItems=1 validation should reject empty subnet lists.
		created = &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-empty-subnets"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{},
			},
		}
		err := k8sClient.Create(ctx, created)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("subnets"))
	})

	It("accepts dual-stack CIDRs on a subnet", func() {
		created = &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-dualstack"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{
						Name:  "web",
						CIDRs: []v1beta1.CIDR{"10.0.1.0/24", "fd00:1::/64"},
						Type:  v1beta1.SubnetTypePublic,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())
	})

	It("rejects a CIDR that is not a masked network address", func() {
		and := &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-host-bit"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{Name: "web", CIDRs: []v1beta1.CIDR{"10.0.1.1/24"}, Type: v1beta1.SubnetTypePublic},
				},
			},
		}
		err := k8sClient.Create(ctx, and)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("rejects two CIDRs from the same address family", func() {
		and := &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-two-v4"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{
						Name:  "web",
						CIDRs: []v1beta1.CIDR{"10.0.1.0/24", "10.0.2.0/24"},
						Type:  v1beta1.SubnetTypePublic,
					},
				},
			},
		}
		err := k8sClient.Create(ctx, and)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("rejects a combined AND+subnet name longer than 63 characters", func() {
		and := &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{Name: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CIDRs: []v1beta1.CIDR{"10.0.1.0/24"}, Type: v1beta1.SubnetTypePublic},
				},
			},
		}
		err := k8sClient.Create(ctx, and)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("rejects mutating an immutable subnet field", func() {
		created = &v1beta1.AdministrativeNetworkDomain{
			ObjectMeta: metav1.ObjectMeta{Name: "and-crd-immutable"},
			Spec: v1beta1.AdministrativeNetworkDomainSpec{
				Subnets: []v1beta1.Subnet{
					{Name: "web", CIDRs: []v1beta1.CIDR{"10.0.1.0/24"}, Type: v1beta1.SubnetTypePublic},
				},
			},
		}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(created), created)).To(Succeed())
		created.Spec.Subnets[0].Type = v1beta1.SubnetTypePrivate
		err := k8sClient.Update(ctx, created)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	Describe("DeepCopy", func() {
		It("deep copies AdministrativeNetworkDomain without sharing slice references", func() {
			original := testAND("test-and",
				testSubnet(testSubnetWeb, testCIDRWeb, v1beta1.SubnetTypePublic),
				testSubnet(testSubnetDB, testCIDRDB, v1beta1.SubnetTypePrivate),
			)
			copy := original.DeepCopy()

			Expect(copy).To(Equal(original), "copy should equal original")
			Expect(copy).NotTo(BeIdenticalTo(original), "copy should be a different object")

			// Mutate copy - original must remain unchanged
			copy.Spec.Subnets[0].Name = "modified"
			copy.Spec.Subnets[0].CIDRs[0] = v1beta1.CIDR(testCIDRModified)
			Expect(original.Spec.Subnets[0].Name).To(Equal(testSubnetWeb))
			Expect(original.Spec.Subnets[0].CIDRs[0]).To(Equal(v1beta1.CIDR(testCIDRWeb)))
		})

		It("deep copies nil slices as nil", func() {
			original := testAND("test-nil-subnets")
			copy := original.DeepCopy()
			Expect(copy.Spec.Subnets).To(BeNil(), "nil Subnets should be preserved")
		})

		It("deep copies status conditions without sharing references", func() {
			original := testAND("test-status")
			original.Status.Conditions = []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Reconciled", Message: "all good"},
			}
			copy := original.DeepCopy()

			copy.Status.Conditions[0].Status = metav1.ConditionFalse
			copy.Status.Conditions[0].Message = "modified"
			Expect(original.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue), "original status should not change")
			Expect(original.Status.Conditions[0].Message).To(Equal("all good"), "original message should not change")
		})

		It("deep copies AdministrativeNetworkDomainList", func() {
			original := &v1beta1.AdministrativeNetworkDomainList{
				Items: []v1beta1.AdministrativeNetworkDomain{
					*testAND("and1"),
					*testAND("and2"),
				},
			}
			copy := original.DeepCopy()

			Expect(copy).To(Equal(original), "copy should equal original list")
			copy.Items[0].Name = "modified"
			Expect(original.Items[0].Name).To(Equal("and1"), "original list item should not change")
		})

		It("deep copies with DeepCopyInto", func() {
			original := testAND("test-copyinto", testSubnet(testSubnetWeb, testCIDRWeb, v1beta1.SubnetTypePublic))
			var target v1beta1.AdministrativeNetworkDomain
			original.DeepCopyInto(&target)

			Expect(&target).To(Equal(original), "target should equal original")
			target.Spec.Subnets[0].Name = "modified"
			Expect(original.Spec.Subnets[0].Name).To(Equal(testSubnetWeb), "original should not change after modifying target")
		})

		It("deep copies with DeepCopyObject", func() {
			original := testAND("test-copyobject")
			copyObj := original.DeepCopyObject()

			copyAND, ok := copyObj.(*v1beta1.AdministrativeNetworkDomain)
			Expect(ok).To(BeTrue(), "DeepCopyObject should return *AdministrativeNetworkDomain")
			Expect(copyAND).To(Equal(original))
		})

		It("deep copies Subnet with availability zone", func() {
			original := v1beta1.Subnet{
				Name:  testSubnetWeb,
				CIDRs: []v1beta1.CIDR{v1beta1.CIDR(testCIDRWeb), v1beta1.CIDR(testCIDRIPv6)},
				Type:  v1beta1.SubnetTypePublic,
				AvailabilityZone: v1beta1.AvailabilityZone{
					ClusterSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"region": "us-east"},
					},
					NodeSelector: map[string]string{"zone": "a"},
				},
			}
			copy := original.DeepCopy()

			Expect(copy).To(Equal(&original), "copy should equal original subnet")
			copy.AvailabilityZone.NodeSelector["zone"] = "b"
			Expect(original.AvailabilityZone.NodeSelector["zone"]).To(Equal("a"), "original availability zone should not change")
		})
	})

	Describe("Additional Validation Edge Cases", func() {
		It("accepts an IPv6-only subnet", func() {
			and := testAND("and-ipv6-only", testSubnet(testSubnetWeb, testCIDRIPv6, v1beta1.SubnetTypePublic))
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND with IPv6-only subnet should succeed")
			created = and
		})

		It("accepts a combined name at exactly 63 characters", func() {
			// AND name (31 chars) + hyphen (1) + subnet name (31 chars) = 63 total
			and := testAND("a123456789012345678901234567890",
				testSubnet("b123456789012345678901234567890", testCIDRWeb, v1beta1.SubnetTypePublic),
			)
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND with 63-char combined name should succeed")
			created = and
		})

		It("rejects mutating subnet CIDRs", func() {
			and := testAND("and-mutate-cidr", testSubnet(testSubnetWeb, testCIDRWeb, v1beta1.SubnetTypePublic))
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND should succeed")
			created = and

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(and), and)).To(Succeed())
			and.Spec.Subnets[0].CIDRs = []v1beta1.CIDR{v1beta1.CIDR(testCIDRDB)}
			err := k8sClient.Update(ctx, and)
			Expect(err).To(HaveOccurred(), "mutating subnet CIDRs should fail")
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("allows adding new subnets to existing AND", func() {
			and := testAND("and-add-subnet", testSubnet(testSubnetWeb, testCIDRWeb, v1beta1.SubnetTypePublic))
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND should succeed")
			created = and

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(and), and)).To(Succeed())
			and.Spec.Subnets = append(and.Spec.Subnets,
				testSubnet(testSubnetDB, testCIDRDB, v1beta1.SubnetTypePrivate),
			)
			Expect(k8sClient.Update(ctx, and)).To(Succeed(), "adding new subnet should succeed")
		})

		It("accepts VPNOnly subnet type", func() {
			and := testAND("and-vpnonly", testSubnet("vpn", "10.0.50.0/24", v1beta1.SubnetTypeVPNOnly))
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND with VPNOnly subnet should succeed")
			created = and
		})

		It("accepts subnet with availability zone", func() {
			subnet := testSubnet(testSubnetWeb, testCIDRWeb, v1beta1.SubnetTypePublic)
			subnet.AvailabilityZone = v1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"region": "us-east"},
				},
				NodeSelector: map[string]string{"topology.kubernetes.io/zone": "rack-a"},
			}
			and := testAND("and-with-az", subnet)
			Expect(k8sClient.Create(ctx, and)).To(Succeed(), "creating AND with availability zone should succeed")
			created = and
		})
	})
})
