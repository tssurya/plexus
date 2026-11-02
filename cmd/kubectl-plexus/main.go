package main

import (
	"os"

	"github.com/ovn-kubernetes/plexus/pkg/cli"
)

func main() {
	if err := cli.NewRootCommand(cli.DefaultClientFactory).Execute(); err != nil {
		os.Exit(1)
	}
}
