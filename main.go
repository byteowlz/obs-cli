package main

import (
	"fmt"
	"os"

	"github.com/muesli/obs-cli/cmd"
	"github.com/muesli/obs-cli/internal/client"
)

var version string

func main() {
	// Set version for user-agent
	client.Version = version

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_ = client.Disconnect()
}
