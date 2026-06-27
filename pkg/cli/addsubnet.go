package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

func newAddSubnetCommand() *cobra.Command {
	var cidr string
	var subnetType string

	cmd := &cobra.Command{
		Use:   "add-subnet <network-domain> <subnet-name>",
		Short: "Add a subnet to an AdministrativeNetworkDomain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ndName := args[0]
			subnetName := args[1]

			c, err := getClient()
			if err != nil {
				return err
			}

			and := &v1beta1.AdministrativeNetworkDomain{}
			if err := c.Get(context.TODO(), client.ObjectKey{Name: ndName}, and); err != nil {
				return fmt.Errorf("getting AND %q: %w", ndName, err)
			}

			for _, s := range and.Spec.Subnets {
				if s.Name == subnetName {
					return fmt.Errorf("subnet %q already exists in AND %q", subnetName, ndName)
				}
			}

			newSubnet := v1beta1.Subnet{
				Name:  subnetName,
				CIDRs: []string{cidr},
				Type:  v1beta1.SubnetType(subnetType),
			}

			and.Spec.Subnets = append(and.Spec.Subnets, newSubnet)

			patchBytes, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"subnets": and.Spec.Subnets,
				},
			})
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			if err := c.Patch(context.TODO(), and, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
				return fmt.Errorf("patching AND %q: %w", ndName, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "subnet/%s added to administrativenetworkdomain/%s\n", subnetName, ndName)
			return nil
		},
	}

	cmd.Flags().StringVar(&cidr, "cidr", "", "CIDR for the subnet (required)")
	cmd.Flags().StringVar(&subnetType, "type", "Private", "Subnet type: Public, Private, Isolated, VPNOnly")
	_ = cmd.MarkFlagRequired("cidr")

	return cmd
}
