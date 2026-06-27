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

func newRemoveSubnetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-subnet <network-domain> <subnet-name>",
		Short: "Remove a subnet from an AdministrativeNetworkDomain",
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

			found := false
			remaining := make([]v1beta1.Subnet, 0, len(and.Spec.Subnets))
			for _, s := range and.Spec.Subnets {
				if s.Name == subnetName {
					found = true
					continue
				}
				remaining = append(remaining, s)
			}
			if !found {
				return fmt.Errorf("subnet %q not found in AND %q", subnetName, ndName)
			}

			patchBytes, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"subnets": remaining,
				},
			})
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			if err := c.Patch(context.TODO(), and, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
				return fmt.Errorf("patching AND %q: %w", ndName, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "subnet/%s removed from administrativenetworkdomain/%s\n", subnetName, ndName)
			return nil
		},
	}
	return cmd
}
