package ovnkubernetes

import (
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rav1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/routeadvertisements/v1"
	udnv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	vtepv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"

	andv1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	configv1beta1 "github.com/ovn-kubernetes/plexus/api/plexuscontrollerconfig/v1beta1"
	"github.com/ovn-kubernetes/plexus/internal/backend"
	"github.com/ovn-kubernetes/plexus/internal/multicluster"
)

var nameCounter int64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, atomic.AddInt64(&nameCounter, 1))
}

// staticInventory is a ClusterInventory that returns a fixed cluster list.
type staticInventory struct {
	clusters []multicluster.ClusterInfo
}

func (s *staticInventory) MatchClusters(selector *metav1.LabelSelector) ([]multicluster.ClusterInfo, error) {
	if selector == nil {
		return append([]multicluster.ClusterInfo(nil), s.clusters...), nil
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	var matched []multicluster.ClusterInfo
	for _, ci := range s.clusters {
		if sel.Matches(labels.Set(ci.Labels)) {
			matched = append(matched, ci)
		}
	}
	return matched, nil
}

func (s *staticInventory) GetCluster(name string) (multicluster.ClusterInfo, error) {
	for _, ci := range s.clusters {
		if ci.Name == name {
			return ci, nil
		}
	}
	return multicluster.ClusterInfo{}, fmt.Errorf("cluster %q not found", name)
}

func (s *staticInventory) AllClusters() ([]multicluster.ClusterInfo, error) {
	return append([]multicluster.ClusterInfo(nil), s.clusters...), nil
}

func hubInventory(c client.Client) *staticInventory {
	return &staticInventory{
		clusters: []multicluster.ClusterInfo{{
			Name:   "hub",
			Client: c,
			IsHub:  true,
			Labels: map[string]string{"region": "us-east"},
		}},
	}
}

func hubAndSpokeInventory(hubClient, spokeClient client.Client) *staticInventory {
	return &staticInventory{
		clusters: []multicluster.ClusterInfo{
			{
				Name:   "hub",
				Client: hubClient,
				IsHub:  true,
				Labels: map[string]string{"region": "us-east"},
			},
			{
				Name:   "spoke-west",
				Client: spokeClient,
				IsHub:  false,
				Labels: map[string]string{"region": "us-west"},
			},
		},
	}
}

func newBackend() *OVNKubernetesBackend {
	return New(k8sClient, logr.Discard(), testConfig(), hubInventory(k8sClient))
}

func newBackendWithSpokes() *OVNKubernetesBackend {
	return New(k8sClient, logr.Discard(), testConfig(), hubAndSpokeInventory(k8sClient, k8sClient))
}

func nsName(andName, subnetName string) string {
	return andName + "-" + subnetName
}

func getNamespace(c client.Client, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, ns)
	return ns, err
}

func getCUDN(c client.Client, name string) (*udnv1.ClusterUserDefinedNetwork, error) {
	cudn := &udnv1.ClusterUserDefinedNetwork{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, cudn)
	return cudn, err
}

func getRA(c client.Client, name string) (*rav1.RouteAdvertisements, error) {
	ra := &rav1.RouteAdvertisements{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, ra)
	return ra, err
}

func getVTEP(c client.Client) (*vtepv1.VTEP, error) {
	vtep := &vtepv1.VTEP{}
	err := c.Get(ctx, client.ObjectKey{Name: vtepName}, vtep)
	return vtep, err
}

func markVTEPAccepted() {
	vtep, err := getVTEP(k8sClient)
	Expect(err).NotTo(HaveOccurred())
	apimeta.SetStatusCondition(&vtep.Status.Conditions, metav1.Condition{
		Type:    "Accepted",
		Status:  metav1.ConditionTrue,
		Reason:  "Accepted",
		Message: "accepted",
	})
	Expect(k8sClient.Status().Update(ctx, vtep)).To(Succeed())
}

func markCUDNReady(name string) {
	cudn, err := getCUDN(k8sClient, name)
	Expect(err).NotTo(HaveOccurred())
	apimeta.SetStatusCondition(&cudn.Status.Conditions, metav1.Condition{
		Type:    "NetworkCreated",
		Status:  metav1.ConditionTrue,
		Reason:  "NetworkCreated",
		Message: "created",
	})
	Expect(k8sClient.Status().Update(ctx, cudn)).To(Succeed())
}

func markRAAccepted(name string) {
	ra, err := getRA(k8sClient, name)
	Expect(err).NotTo(HaveOccurred())
	apimeta.SetStatusCondition(&ra.Status.Conditions, metav1.Condition{
		Type:    rav1.RouteAdvertisementsAccepted,
		Status:  metav1.ConditionTrue,
		Reason:  "Accepted",
		Message: "accepted",
	})
	Expect(k8sClient.Status().Update(ctx, ra)).To(Succeed())
}

func namespaceDeletedOrTerminating(name string) bool {
	ns, err := getNamespace(k8sClient, name)
	if apierrors.IsNotFound(err) {
		return true
	}
	return err == nil && ns.DeletionTimestamp != nil
}

func cleanupClusterResources() {
	_ = k8sClient.DeleteAllOf(ctx, &andv1beta1.AdministrativeNetworkDomain{})
	_ = k8sClient.DeleteAllOf(ctx, &udnv1.ClusterUserDefinedNetwork{})
	_ = k8sClient.DeleteAllOf(ctx, &rav1.RouteAdvertisements{})
	_ = k8sClient.DeleteAllOf(ctx, &vtepv1.VTEP{})

	// Namespaces are requested for deletion but never actually disappear:
	// envtest runs only kube-apiserver and etcd, with no namespace
	// controller to drain contents and drop the "kubernetes" finalizer, so
	// every namespace stays Terminating for the lifetime of the suite.
	// Waiting for deletion here would therefore hang until timeout. Specs
	// keep isolation by giving each AND a unique name (see uniqueName), so
	// leftover namespaces from a previous spec can never collide; the
	// delete below is housekeeping, and assertions about namespace removal
	// use namespaceDeletedOrTerminating instead of IsNotFound.
	var nsList corev1.NamespaceList
	if err := k8sClient.List(ctx, &nsList, client.HasLabels{labelNetworkDomain}); err == nil {
		for i := range nsList.Items {
			_ = k8sClient.Delete(ctx, &nsList.Items[i])
		}
	}
}

// Test constants for common subnet configurations
const (
	testSubnetWeb    = "web"
	testSubnetDB     = "db"
	testCIDRWeb      = "10.0.1.0/24"
	testCIDRDB       = "10.0.2.0/24"
	testCIDRIsolated = "10.0.20.0/24"
	testCIDRIPv6     = "fd00:10::/64"
	testCIDRIPv6Alt  = "fd00:20::/64"
)

// reconcileNewAND creates a new backend, AND, and performs initial reconciliation.
// This helper reduces duplication in status check tests.
func reconcileNewAND(subnetType andv1beta1.SubnetType) (*OVNKubernetesBackend, *andv1beta1.AdministrativeNetworkDomain, error) {
	b := newBackend()
	and := testAND(uniqueName("and"), testSubnet(testSubnetWeb, testCIDRWeb, subnetType))
	_, err := b.Reconcile(ctx, and)
	return b, and, err
}

// setConditionFalse is a helper to set a status condition to False on various resource types.
func setConditionFalse(obj client.Object, condType, reason, message string) {
	cond := metav1.Condition{
		Type:    condType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	}

	switch v := obj.(type) {
	case *vtepv1.VTEP:
		apimeta.SetStatusCondition(&v.Status.Conditions, cond)
	case *udnv1.ClusterUserDefinedNetwork:
		apimeta.SetStatusCondition(&v.Status.Conditions, cond)
	case *rav1.RouteAdvertisements:
		apimeta.SetStatusCondition(&v.Status.Conditions, cond)
	}
}

var _ = Describe("OVNKubernetesBackend", func() {
	It("returns ovn-kubernetes from Name", func() {
		b := New(nil, logr.Discard(), testConfig(), &staticInventory{})
		Expect(b.Name()).To(Equal("ovn-kubernetes"))
	})
})

var _ = Describe("OVNKubernetesBackend resources", func() {
	BeforeEach(func() {
		if k8sClient == nil {
			Skip("KUBEBUILDER_ASSETS not set — run 'make test' to include envtest")
		}
		cleanupClusterResources()
	})

	AfterEach(func() {
		if k8sClient != nil {
			cleanupClusterResources()
		}
	})

	Describe("Namespace", func() {
		It("creates a namespace per subnet with plexus labels", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			ns, err := getNamespace(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name))
			Expect(ns.Labels).To(HaveKeyWithValue(labelSubnet, "web"))
			Expect(ns.Labels).To(HaveKeyWithValue(labelSubnetType, string(andv1beta1.SubnetTypePublic)))
			Expect(ns.Labels).To(HaveKeyWithValue(labelPrimaryUDN, ""))
		})

		It("sets the node-selector annotation from the availability zone", func() {
			b := newBackend()
			subnet := testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic)
			subnet.AvailabilityZone = andv1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{},
				NodeSelector: map[string]string{
					"topology.kubernetes.io/zone": "rack-a",
					"node-role":                   "worker",
				},
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			ns, err := getNamespace(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Annotations).To(HaveKeyWithValue(
				annotationNodeSelector,
				"node-role=worker,topology.kubernetes.io/zone=rack-a",
			))
		})

		It("repairs drifted namespace labels and removes a stale node-selector annotation", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePrivate))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			ns, err := getNamespace(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			ns.Labels[labelSubnetType] = "wrong"
			if ns.Annotations == nil {
				ns.Annotations = map[string]string{}
			}
			ns.Annotations[annotationNodeSelector] = "stale=true"
			Expect(k8sClient.Update(ctx, ns)).To(Succeed())

			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			ns, err = getNamespace(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Labels).To(HaveKeyWithValue(labelSubnetType, string(andv1beta1.SubnetTypePrivate)))
			_, hasAnnotation := ns.Annotations[annotationNodeSelector]
			Expect(hasAnnotation).To(BeFalse())
		})
	})

	Describe("ClusterUserDefinedNetwork", func() {
		It("creates a Layer2 EVPN CUDN for a Public subnet with MACVRF and IPVRF", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cudn.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name))
			Expect(cudn.Labels).To(HaveKeyWithValue(labelSubnet, "web"))
			Expect(cudn.Spec.NamespaceSelector.MatchLabels).To(SatisfyAll(
				HaveKeyWithValue(labelNetworkDomain, and.Name),
				HaveKeyWithValue(labelSubnet, "web"),
			))
			Expect(cudn.Spec.Network.Topology).To(Equal(udnv1.NetworkTopologyLayer2))
			Expect(cudn.Spec.Network.Transport).To(Equal(udnv1.TransportOptionEVPN))
			Expect(cudn.Spec.Network.Layer2).NotTo(BeNil())
			Expect(cudn.Spec.Network.Layer2.Role).To(Equal(udnv1.NetworkRolePrimary))
			Expect(cudn.Spec.Network.Layer2.Subnets).To(ConsistOf(udnv1.CIDR("10.0.1.0/24")))
			Expect(cudn.Spec.Network.EVPN).NotTo(BeNil())
			Expect(cudn.Spec.Network.EVPN.VTEP).To(Equal(vtepName))
			Expect(cudn.Spec.Network.EVPN.MACVRF).NotTo(BeNil())
			Expect(cudn.Spec.Network.EVPN.MACVRF.VNI).To(BeNumerically(">=", 4096))
			Expect(cudn.Spec.Network.EVPN.IPVRF).NotTo(BeNil())
			Expect(cudn.Spec.Network.EVPN.IPVRF.VNI).To(BeNumerically(">=", 4096))
			Expect(cudn.Spec.Network.EVPN.IPVRF.VNI).NotTo(Equal(cudn.Spec.Network.EVPN.MACVRF.VNI))
			Expect(string(cudn.Spec.Network.EVPN.IPVRF.RouteTarget)).To(HavePrefix("*:"))
		})

		It("omits IPVRF on Isolated subnet CUDNs", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("db", "10.0.20.0/24", andv1beta1.SubnetTypeIsolated))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "db"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cudn.Spec.Network.EVPN.MACVRF).NotTo(BeNil())
			Expect(cudn.Spec.Network.EVPN.IPVRF).To(BeNil())
		})

		It("includes IPVRF on VPNOnly subnet CUDNs", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("vpn", "10.0.30.0/24", andv1beta1.SubnetTypeVPNOnly))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "vpn"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cudn.Spec.Network.EVPN.IPVRF).NotTo(BeNil())
			Expect(cudn.Spec.Network.EVPN.IPVRF.VNI).To(BeNumerically(">=", 4096))
		})

		It("repairs missing CUDN labels on a subsequent reconcile", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypeIsolated))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			cudn.Labels = map[string]string{}
			Expect(k8sClient.Update(ctx, cudn)).To(Succeed())

			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cudn, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cudn.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name))
			Expect(cudn.Labels).To(HaveKeyWithValue(labelSubnet, "web"))
			Expect(cudn.Labels).To(HaveKeyWithValue(labelSubnetType, string(andv1beta1.SubnetTypeIsolated)))
		})

		It("shares the same route target across non-Isolated subnets of one AND", func() {
			b := newBackend()
			and := testAND(uniqueName("and"),
				testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic),
				testSubnet("app", "10.0.10.0/24", andv1beta1.SubnetTypePrivate),
			)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			web, err := getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			app, err := getCUDN(k8sClient, nsName(and.Name, "app"))
			Expect(err).NotTo(HaveOccurred())
			Expect(web.Spec.Network.EVPN.IPVRF.RouteTarget).To(Equal(app.Spec.Network.EVPN.IPVRF.RouteTarget))
		})

		It("preserves VNIs when a fresh backend reconciles an existing CUDN", func() {
			b1 := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b1.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			original, err := getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			macVNI := original.Spec.Network.EVPN.MACVRF.VNI
			ipVNI := original.Spec.Network.EVPN.IPVRF.VNI

			b2 := newBackend()
			_, err = b2.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			updated, err := getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Spec.Network.EVPN.MACVRF.VNI).To(Equal(macVNI))
			Expect(updated.Spec.Network.EVPN.IPVRF.VNI).To(Equal(ipVNI))
		})

		It("creates a CUDN with IPv6-only subnet", func() {
			b := newBackend()
			subnet := testSubnet("ipv6-net", testCIDRIPv6, andv1beta1.SubnetTypePublic)
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconciling AND with IPv6-only subnet should succeed")

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "ipv6-net"))
			Expect(err).NotTo(HaveOccurred(), "CUDN should be created for IPv6 subnet")
			Expect(cudn.Spec.Network.Layer2.Subnets).To(ConsistOf(udnv1.CIDR(testCIDRIPv6)),
				"CUDN should contain the IPv6 CIDR")
			Expect(cudn.Spec.Network.EVPN.MACVRF).NotTo(BeNil(), "MACVRF should be allocated for IPv6")
			Expect(cudn.Spec.Network.EVPN.IPVRF).NotTo(BeNil(), "IPVRF should be allocated for IPv6")
		})

		It("creates a CUDN with dual-stack (IPv4+IPv6) subnet", func() {
			b := newBackend()
			subnet := &andv1beta1.Subnet{
				Name:  "dual-stack",
				CIDRs: []andv1beta1.CIDR{andv1beta1.CIDR(testCIDRWeb), andv1beta1.CIDR(testCIDRIPv6)},
				Type:  andv1beta1.SubnetTypePublic,
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconciling AND with dual-stack subnet should succeed")

			cudn, err := getCUDN(k8sClient, nsName(and.Name, "dual-stack"))
			Expect(err).NotTo(HaveOccurred(), "CUDN should be created for dual-stack subnet")
			Expect(cudn.Spec.Network.Layer2.Subnets).To(ConsistOf(
				udnv1.CIDR(testCIDRWeb),
				udnv1.CIDR(testCIDRIPv6),
			), "CUDN should contain both IPv4 and IPv6 CIDRs")
			Expect(cudn.Spec.Network.EVPN.MACVRF).NotTo(BeNil(), "MACVRF should be allocated for dual-stack")
			Expect(cudn.Spec.Network.EVPN.IPVRF).NotTo(BeNil(), "IPVRF should be allocated for dual-stack")
		})

		It("creates namespace labels correctly for IPv6 subnet", func() {
			b := newBackend()
			subnet := testSubnet("ipv6-subnet", testCIDRIPv6, andv1beta1.SubnetTypePrivate)
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconciling AND with IPv6 subnet should succeed")

			ns, err := getNamespace(k8sClient, nsName(and.Name, "ipv6-subnet"))
			Expect(err).NotTo(HaveOccurred(), "namespace should be created for IPv6 subnet")
			Expect(ns.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name),
				"namespace should have network domain label")
			Expect(ns.Labels).To(HaveKeyWithValue(labelSubnet, "ipv6-subnet"),
				"namespace should have subnet label")
			Expect(ns.Labels).To(HaveKeyWithValue(labelSubnetType, string(andv1beta1.SubnetTypePrivate)),
				"namespace should have correct subnet type label")
		})
	})

	Describe("RouteAdvertisements", func() {
		It("creates a RouteAdvertisements selecting Public and Private CUDNs", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			ra, err := getRA(k8sClient, and.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(ra.Labels).To(HaveKeyWithValue(labelManagedBy, "plexus"))
			Expect(ra.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name))
			Expect(ra.Spec.TargetVRF).To(Equal("auto"))
			Expect(ra.Spec.Advertisements).To(Equal([]rav1.AdvertisementType{rav1.PodNetwork}))
			Expect(ra.Spec.FRRConfigurationSelector.MatchLabels).To(HaveKeyWithValue("app", "frr"))
			Expect(ra.Spec.NetworkSelectors).To(HaveLen(1))
			sel := ra.Spec.NetworkSelectors[0].ClusterUserDefinedNetworkSelector
			Expect(sel).NotTo(BeNil())
			Expect(sel.NetworkSelector.MatchLabels).To(HaveKeyWithValue(labelNetworkDomain, and.Name))
			Expect(sel.NetworkSelector.MatchExpressions).To(HaveLen(1))
			req := sel.NetworkSelector.MatchExpressions[0]
			Expect(req.Key).To(Equal(labelSubnetType))
			Expect(req.Operator).To(Equal(metav1.LabelSelectorOpIn))
			// Values is a set as far as the selector is concerned, so the
			// order it is built in must not make this assertion fail.
			Expect(req.Values).To(ConsistOf(
				string(andv1beta1.SubnetTypePublic),
				string(andv1beta1.SubnetTypePrivate),
			))
		})

		It("does not create RouteAdvertisements when every subnet is Isolated", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("db", "10.0.20.0/24", andv1beta1.SubnetTypeIsolated))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			_, err = getRA(k8sClient, and.Name)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("deletes RouteAdvertisements after the last non-Isolated subnet is removed", func() {
			b := newBackend()
			and := testAND(uniqueName("and"),
				testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic),
				testSubnet("db", "10.0.20.0/24", andv1beta1.SubnetTypeIsolated),
			)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			_, err = getRA(k8sClient, and.Name)
			Expect(err).NotTo(HaveOccurred())

			and.Spec.Subnets = []andv1beta1.Subnet{*testSubnet("db", "10.0.20.0/24", andv1beta1.SubnetTypeIsolated)}
			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			_, err = getRA(k8sClient, and.Name)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("creates RouteAdvertisements for IPv6 Public subnet", func() {
			b := newBackend()
			subnet := testSubnet("ipv6-public", testCIDRIPv6, andv1beta1.SubnetTypePublic)
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconciling AND with IPv6 Public subnet should succeed")

			ra, err := getRA(k8sClient, and.Name)
			Expect(err).NotTo(HaveOccurred(), "RouteAdvertisements should be created for IPv6 Public subnet")
			Expect(ra.Labels).To(HaveKeyWithValue(labelNetworkDomain, and.Name),
				"RouteAdvertisements should reference the AND")
			Expect(ra.Spec.NetworkSelectors).To(HaveLen(1),
				"RouteAdvertisements should select CUDNs for IPv6 subnet")
		})

		It("creates RouteAdvertisements for dual-stack Public subnet", func() {
			b := newBackend()
			subnet := &andv1beta1.Subnet{
				Name:  "dual-public",
				CIDRs: []andv1beta1.CIDR{andv1beta1.CIDR(testCIDRWeb), andv1beta1.CIDR(testCIDRIPv6)},
				Type:  andv1beta1.SubnetTypePublic,
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconciling AND with dual-stack Public subnet should succeed")

			ra, err := getRA(k8sClient, and.Name)
			Expect(err).NotTo(HaveOccurred(), "RouteAdvertisements should be created for dual-stack subnet")
			Expect(ra.Spec.Advertisements).To(ContainElement(rav1.PodNetwork),
				"RouteAdvertisements should advertise pod networks for dual-stack")
		})
	})

	Describe("VTEP", func() {
		It("creates the shared unmanaged VTEP with configured CIDRs", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			vtep, err := getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(vtep.Labels).To(HaveKeyWithValue(labelManagedBy, "plexus"))
			Expect(vtep.Spec.Mode).To(Equal(vtepv1.VTEPModeUnmanaged))
			Expect(vtep.Spec.CIDRs).To(ConsistOf(vtepv1.CIDR("172.18.0.0/16")))
		})

		It("updates VTEP CIDRs when the backend config changes", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			cfg := testConfig()
			cfg.VTEPCIDRs = []configv1beta1.CIDR{"10.100.0.0/16"}
			updated := New(k8sClient, logr.Discard(), cfg, hubInventory(k8sClient))
			_, err = updated.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			vtep, err := getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(vtep.Spec.CIDRs).To(ConsistOf(vtepv1.CIDR("10.100.0.0/16")))
		})
	})

	Describe("status aggregation", func() {
		It("reports VTEPNotReady until the VTEP is accepted", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("VTEPNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("VTEP not yet accepted"))
		})

		It("reports SubnetsNotReady until CUDNs and RouteAdvertisements are accepted", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			markVTEPAccepted()
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("network not yet created"))

			markCUDNReady(nsName(and.Name, "web"))
			result, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("RouteAdvertisements not yet accepted"))

			markRAAccepted(and.Name)
			result, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(backend.Result{}))
		})
	})

	Describe("garbage collection", func() {
		It("deletes Namespace and CUDN for a subnet removed from the AND spec", func() {
			b := newBackend()
			and := testAND(uniqueName("and"),
				testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic),
				testSubnet("app", "10.0.10.0/24", andv1beta1.SubnetTypePrivate),
			)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())
			_, err = getCUDN(k8sClient, nsName(and.Name, "app"))
			Expect(err).NotTo(HaveOccurred())

			and.Spec.Subnets = []andv1beta1.Subnet{*testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic)}
			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			_, err = getCUDN(k8sClient, nsName(and.Name, "app"))
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(namespaceDeletedOrTerminating(nsName(and.Name, "app"))).To(BeTrue())

			_, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Delete", func() {
		It("removes AND-scoped CUDNs, RouteAdvertisements, namespaces, and the shared VTEP", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			Expect(k8sClient.Create(ctx, and.DeepCopy())).To(Succeed())
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			Expect(b.Delete(ctx, and)).To(Succeed())

			_, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			_, err = getRA(k8sClient, and.Name)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(namespaceDeletedOrTerminating(nsName(and.Name, "web"))).To(BeTrue())
			_, err = getVTEP(k8sClient)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("keeps the shared VTEP while another AND still exists", func() {
			b := newBackend()
			andA := testAND(uniqueName("and"), testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic))
			andB := testAND(uniqueName("and"), testSubnet("app", "10.0.10.0/24", andv1beta1.SubnetTypePrivate))
			Expect(k8sClient.Create(ctx, andA.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, andB.DeepCopy())).To(Succeed())

			_, err := b.Reconcile(ctx, andA)
			Expect(err).NotTo(HaveOccurred())
			_, err = b.Reconcile(ctx, andB)
			Expect(err).NotTo(HaveOccurred())

			Expect(b.Delete(ctx, andA)).To(Succeed())

			_, err = getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred())
			_, err = getCUDN(k8sClient, nsName(andA.Name, "web"))
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			_, err = getCUDN(k8sClient, nsName(andB.Name, "app"))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("cluster selector", func() {
		It("renders subnet resources when the hub matches the availability zone", func() {
			b := newBackend()
			subnet := testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic)
			subnet.AvailabilityZone = andv1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"region": "us-east"},
				},
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			_, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips subnet resources on clusters that do not match the availability zone", func() {
			b := newBackend()
			subnet := testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic)
			subnet.AvailabilityZone = andv1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"region": "us-west"},
				},
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			_, err = getNamespace(k8sClient, nsName(and.Name, "web"))
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			_, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			// Shared VTEP is still reconciled on every cluster in the inventory.
			_, err = getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("multi-cluster with spoke", func() {
		It("creates subnet resources when spoke inventory has matching cluster", func() {
			b := newBackendWithSpokes()
			subnet := testSubnet("web", "10.0.1.0/24", andv1beta1.SubnetTypePublic)
			subnet.AvailabilityZone = andv1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"region": "us-west"},
				},
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			// Verify resource reconciliation succeeded with spoke inventory
			// (In a real multi-cluster scenario with separate clients, we could verify
			// resources on the specific spoke client, but here we verify the reconciliation
			// succeeded, which means it correctly identified the spoke cluster matches)
			_, err = getCUDN(k8sClient, nsName(and.Name, "web"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("reconciles resources on hub when selector matches hub labels in spoke inventory", func() {
			b := newBackendWithSpokes()
			subnet := testSubnet("app", "10.0.10.0/24", andv1beta1.SubnetTypePrivate)
			subnet.AvailabilityZone = andv1beta1.AvailabilityZone{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"region": "us-east"},
				},
			}
			and := testAND(uniqueName("and"), subnet)
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred())

			// Verify hub resources were created
			_, err = getCUDN(k8sClient, nsName(and.Name, "app"))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Status Checking", func() {
		It("returns VTEPNotReady when VTEP Accepted condition is missing", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			// VTEP exists but has no status conditions
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with VTEP not ready")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("VTEPNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("VTEP not yet accepted"))
		})

		It("returns VTEPNotReady when VTEP Accepted=False", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			vtep, err := getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred(), "VTEP should exist after reconciliation")
			setConditionFalse(vtep, "Accepted", "InvalidConfig", "VTEP configuration is invalid")
			Expect(k8sClient.Status().Update(ctx, vtep)).To(Succeed(), "updating VTEP status should succeed")

			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with VTEP not accepted")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("VTEPNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("VTEP configuration is invalid"))
		})

		It("returns SubnetsNotReady when CUDN NetworkCreated condition is missing", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")
			markVTEPAccepted()

			// CUDN exists but has no NetworkCreated condition
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with CUDN not ready")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("network not yet created"))
		})

		It("returns SubnetsNotReady when CUDN NetworkCreated=False", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")
			markVTEPAccepted()

			cudn, err := getCUDN(k8sClient, nsName(and.Name, testSubnetWeb))
			Expect(err).NotTo(HaveOccurred(), "CUDN should exist after reconciliation")
			setConditionFalse(cudn, "NetworkCreated", "CreationFailed", "failed to create network resources")
			Expect(k8sClient.Status().Update(ctx, cudn)).To(Succeed(), "updating CUDN status should succeed")

			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with CUDN creation failed")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("failed to create network resources"))
		})

		It("returns SubnetsNotReady when RouteAdvertisements Accepted condition is missing", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")
			markVTEPAccepted()
			markCUDNReady(nsName(and.Name, testSubnetWeb))

			// RouteAdvertisements exists but has no Accepted condition
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with RA not ready")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("RouteAdvertisements not yet accepted"))
		})

		It("returns SubnetsNotReady when RouteAdvertisements Accepted=False", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")
			markVTEPAccepted()
			markCUDNReady(nsName(and.Name, testSubnetWeb))

			ra, err := getRA(k8sClient, raName(and))
			Expect(err).NotTo(HaveOccurred(), "RouteAdvertisements should exist after reconciliation")
			setConditionFalse(ra, rav1.RouteAdvertisementsAccepted, "InvalidAdvertisement", "route advertisement configuration is invalid")
			Expect(k8sClient.Status().Update(ctx, ra)).To(Succeed(), "updating RouteAdvertisements status should succeed")

			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed even with RA not accepted")
			Expect(result.Requeue).To(BeTrue())
			Expect(result.StatusReason).To(Equal("SubnetsNotReady"))
			Expect(result.StatusMessage).To(ContainSubstring("route advertisement configuration is invalid"))
		})

		It("returns ready status when all resources are ready", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			// Mark all resources as ready
			markVTEPAccepted()
			markCUDNReady(nsName(and.Name, testSubnetWeb))
			markRAAccepted(raName(and))

			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed when all resources ready")
			Expect(result.Requeue).To(BeFalse())
			Expect(result.StatusReason).To(BeEmpty())
			Expect(result.StatusMessage).To(BeEmpty())
		})

		It("recreates VTEP if deleted", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			// Delete VTEP
			vtep, err := getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred(), "VTEP should exist after initial reconciliation")
			Expect(k8sClient.Delete(ctx, vtep)).To(Succeed(), "deleting VTEP should succeed")

			// Reconcile should recreate it
			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should recreate deleted VTEP")

			vtep, err = getVTEP(k8sClient)
			Expect(err).NotTo(HaveOccurred(), "VTEP should exist after recreation")
			Expect(vtep.Spec.CIDRs).To(Equal([]vtepv1.CIDR{"172.18.0.0/16"}))
		})

		It("recreates CUDN if deleted", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			// Delete CUDN
			cudn, err := getCUDN(k8sClient, nsName(and.Name, testSubnetWeb))
			Expect(err).NotTo(HaveOccurred(), "CUDN should exist after initial reconciliation")
			Expect(k8sClient.Delete(ctx, cudn)).To(Succeed(), "deleting CUDN should succeed")

			// Reconcile should recreate it
			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should recreate deleted CUDN")

			cudn, err = getCUDN(k8sClient, nsName(and.Name, testSubnetWeb))
			Expect(err).NotTo(HaveOccurred(), "CUDN should exist after recreation")
			Expect(cudn.Spec.Network.Topology).To(Equal(udnv1.NetworkTopologyLayer2))
		})

		It("recreates RouteAdvertisements if deleted", func() {
			b, and, err := reconcileNewAND(andv1beta1.SubnetTypePublic)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			// Delete RouteAdvertisements
			ra, err := getRA(k8sClient, raName(and))
			Expect(err).NotTo(HaveOccurred(), "RouteAdvertisements should exist after initial reconciliation")
			Expect(k8sClient.Delete(ctx, ra)).To(Succeed(), "deleting RouteAdvertisements should succeed")

			// Reconcile should recreate it
			_, err = b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should recreate deleted RouteAdvertisements")

			ra, err = getRA(k8sClient, raName(and))
			Expect(err).NotTo(HaveOccurred(), "RouteAdvertisements should exist after recreation")
			Expect(ra.Name).To(Equal(raName(and)))
		})

		It("does not check RouteAdvertisements for Isolated-only ANDs", func() {
			b := newBackend()
			and := testAND(uniqueName("and"), testSubnet(testSubnetDB, testCIDRIsolated, andv1beta1.SubnetTypeIsolated))
			_, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "initial reconciliation should succeed")

			markVTEPAccepted()
			markCUDNReady(nsName(and.Name, testSubnetDB))

			// No RouteAdvertisements should be created for Isolated subnets
			_, err = getRA(k8sClient, raName(and))
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "RouteAdvertisements should not exist for Isolated subnet")

			// Status check should succeed without checking RA
			result, err := b.Reconcile(ctx, and)
			Expect(err).NotTo(HaveOccurred(), "reconcile should succeed for Isolated subnet without RA")
			Expect(result.Requeue).To(BeFalse())
		})
	})
})
