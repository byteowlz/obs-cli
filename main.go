package main

import (
	"fmt"
	"os"

	"github.com/muesli/obs-cli/cmd"
	"github.com/muesli/obs-cli/internal/client"
)

var version = "dev"

func main() {
	// Set version for user-agent
	client.Version = version
	cmd.RootCmd.Version = version
	// Register before command discovery so mixed connection/version flags parse correctly.
	cmd.RootCmd.InitDefaultVersionFlag()

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_ = client.Disconnect()
}
