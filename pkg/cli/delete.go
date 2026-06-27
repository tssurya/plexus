package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an AdministrativeNetworkDomain and all its subnets",
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
			}

			if err := c.Delete(context.TODO(), and); err != nil {
				return fmt.Errorf("deleting AND %q: %w", args[0], err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "administrativenetworkdomain/%s deleted\n", args[0])
			return nil
		},
	}
	return cmd
}
