package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an empty AdministrativeNetworkDomain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}

			and := &v1beta1.AdministrativeNetworkDomain{
				ObjectMeta: metav1.ObjectMeta{
					Name: args[0],
				},
				Spec: v1beta1.AdministrativeNetworkDomainSpec{
					Subnets: []v1beta1.Subnet{},
				},
			}

			if err := c.Create(context.TODO(), and); err != nil {
				return fmt.Errorf("creating AND %q: %w", args[0], err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "administrativenetworkdomain/%s created\n", args[0])
			return nil
		},
	}
	return cmd
}
