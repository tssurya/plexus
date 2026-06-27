package main

import (
	"os"

	"github.com/ovn-kubernetes/plexus/pkg/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
