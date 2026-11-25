package ovnkubernetes

import (
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	andv1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
	configv1beta1 "github.com/ovn-kubernetes/plexus/api/plexuscontrollerconfig/v1beta1"
)

// Fixture builders for the backend tests, and the only ones in this package:
// add to these rather than defining a second set in a spec file.
//
// Every test package names its fixtures testX for consistency. This package
// is the one that returns *Subnet rather than Subnet, because the production
// helpers it exercises (buildCUDN, desiredLabels, namespaceName, ...) all take
// *Subnet, so the fixtures compose inline at those call sites. Keep it that
// way; matching the package under test beats matching the other packages.

// testConfig returns the default OVN-Kubernetes backend configuration for tests.
func testConfig() *configv1beta1.OVNKubernetesConfig {
	return &configv1beta1.OVNKubernetesConfig{
		VTEPCIDRs: []configv1beta1.CIDR{"172.18.0.0/16"},
		FRRConfigurationSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "frr"},
		},
	}
}

func testBackend() *OVNKubernetesBackend {
	return New(nil, logr.Discard(), testConfig(), nil)
}

// testAND builds an AND fixture. Subnets are taken as pointers so that
// testSubnet composes inline here and at the production helpers (buildCUDN,
// desiredLabels, ...), which all take *Subnet. One fixture API, one shape.
func testAND(name string, subnets ...*andv1beta1.Subnet) *andv1beta1.AdministrativeNetworkDomain {
	var spec []andv1beta1.Subnet
	for _, s := range subnets {
		spec = append(spec, *s)
	}
	return &andv1beta1.AdministrativeNetworkDomain{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       andv1beta1.AdministrativeNetworkDomainSpec{Subnets: spec},
	}
}

func testSubnet(name, cidr string, subnetType andv1beta1.SubnetType) *andv1beta1.Subnet {
	return &andv1beta1.Subnet{
		Name:  name,
		CIDRs: []andv1beta1.CIDR{andv1beta1.CIDR(cidr)},
		Type:  subnetType,
	}
}
