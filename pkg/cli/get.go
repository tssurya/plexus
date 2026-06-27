package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

func newGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "List all AdministrativeNetworkDomains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}

			list := &v1beta1.AdministrativeNetworkDomainList{}
			if err := c.List(context.TODO(), list); err != nil {
				return fmt.Errorf("listing ANDs: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSUBNETS\tREADY")
			for _, and := range list.Items {
				ready := "Unknown"
				if cond := meta.FindStatusCondition(and.Status.Conditions, "Ready"); cond != nil {
					ready = string(cond.Status)
				}
				fmt.Fprintf(w, "%s\t%d\t%s\n", and.Name, len(and.Spec.Subnets), ready)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}
