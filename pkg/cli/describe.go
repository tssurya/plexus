package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/ovn-kubernetes/plexus/api/administrativenetworkdomain/v1beta1"
)

func newDescribeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <name>",
		Short: "Show details of an AdministrativeNetworkDomain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}

			and := &v1beta1.AdministrativeNetworkDomain{}
			if err := c.Get(context.TODO(), client.ObjectKey{Name: args[0]}, and); err != nil {
				return fmt.Errorf("getting AND %q: %w", args[0], err)
			}

			ready := "Unknown"
			if cond := meta.FindStatusCondition(and.Status.Conditions, "Ready"); cond != nil {
				ready = string(cond.Status)
			}

			fmt.Fprintf(os.Stdout, "Name:    %s\n", and.Name)
			fmt.Fprintf(os.Stdout, "Ready:   %s\n", ready)
			fmt.Fprintf(os.Stdout, "Subnets: %d\n\n", len(and.Spec.Subnets))

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SUBNET\tCIDRS\tTYPE\tAZ")
			for _, s := range and.Spec.Subnets {
				az := "<none>"
				if s.AvailabilityZone != nil {
					parts := []string{}
					for k, v := range s.AvailabilityZone.NodeSelector {
						parts = append(parts, fmt.Sprintf("%s=%s", k, v))
					}
					if len(parts) > 0 {
						az = strings.Join(parts, ",")
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, strings.Join(s.CIDRs, ","), s.Type, az)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}
