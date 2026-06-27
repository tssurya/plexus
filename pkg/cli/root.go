package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plexus",
		Short: "Plexus — manage AdministrativeNetworkDomains",
		Long: `Plexus is a network orchestrator that provides VPC-like isolation
on Kubernetes. Use this CLI to create, manage, and inspect
AdministrativeNetworkDomains (ANDs) and their subnets.

This command can be used standalone or as a kubectl plugin (kubectl plexus).`,
	}

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newDeleteCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newDescribeCommand())
	cmd.AddCommand(newAddSubnetCommand())
	cmd.AddCommand(newRemoveSubnetCommand())

	return cmd
}
