package cli

import (
	"fmt"
	"net"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

var validSubnetTypes = map[v1beta1.SubnetType]bool{
	v1beta1.SubnetTypePublic:   true,
	v1beta1.SubnetTypePrivate:  true,
	v1beta1.SubnetTypeIsolated: true,
	v1beta1.SubnetTypeVPNOnly:  true,
}

func validateSubnetType(t string) (v1beta1.SubnetType, error) {
	st := v1beta1.SubnetType(t)
	if !validSubnetTypes[st] {
		return "", fmt.Errorf("invalid subnet type %q: must be one of Public, Private, Isolated, VPNOnly", t)
	}
	return st, nil
}

func validateCIDRs(cidrs []string) error {
	if len(cidrs) == 0 {
		return fmt.Errorf("at least one CIDR is required")
	}
	if len(cidrs) > 2 {
		return fmt.Errorf("at most two CIDRs are allowed (one IPv4, one IPv6 for dual-stack)")
	}

	hasV4, hasV6 := false, false
	for _, cidr := range cidrs {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		if ip.To4() != nil {
			if hasV4 {
				return fmt.Errorf("two IPv4 CIDRs specified: dual-stack requires one IPv4 and one IPv6")
			}
			hasV4 = true
		} else {
			if hasV6 {
				return fmt.Errorf("two IPv6 CIDRs specified: dual-stack requires one IPv4 and one IPv6")
			}
			hasV6 = true
		}
	}
	return nil
}
