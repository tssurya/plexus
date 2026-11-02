package cli

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spf13/cobra"
)

// ClientFactory builds the Kubernetes client a command talks to. Commands take
// one instead of calling getClient directly so tests can inject a fake.
type ClientFactory func() (client.Client, error)

// DefaultClientFactory builds a real client from the --kubeconfig/--context
// flags. Production entrypoints pass this; tests pass a fake.
var DefaultClientFactory ClientFactory = getClient

var (
	kubeconfig  string
	kubecontext string
)

// NewRootCommand builds the root command, wiring clientFn into every
// subcommand. Pass DefaultClientFactory outside of tests.
func NewRootCommand(clientFn ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plexus",
		Short: "Plexus — manage AdministrativeNetworkDomains",
		Long: `Plexus is a network orchestrator that provides VPC-like isolation
on Kubernetes. Use this CLI to create, manage, and inspect
AdministrativeNetworkDomains (ANDs) and their subnets.

This command can be used standalone or as a kubectl plugin (kubectl plexus).`,
	}

	cmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	cmd.PersistentFlags().StringVar(&kubecontext, "context", "", "The name of the kubeconfig context to use")

	cmd.AddCommand(newCreateCommand(clientFn))
	cmd.AddCommand(newDeleteCommand(clientFn))
	cmd.AddCommand(newDescribeCommand(clientFn))
	cmd.AddCommand(newAddSubnetCommand(clientFn))
	cmd.AddCommand(newDeleteSubnetCommand(clientFn))
	cmd.AddCommand(newVersionCommand())

	return cmd
}
