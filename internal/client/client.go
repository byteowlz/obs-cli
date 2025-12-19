package client

import (
	"fmt"
	"net/http"

	"github.com/andreykaipov/goobs"
)

var (
	// Client is the global OBS WebSocket client
	Client *goobs.Client

	// Version is set at build time
	Version string
)

// Connect establishes a connection to the OBS WebSocket server
func Connect(host string, port uint32, password string) error {
	var err error
	Client, err = goobs.New(
		host+fmt.Sprintf(":%d", port),
		goobs.WithPassword(password),
		goobs.WithRequestHeader(http.Header{"User-Agent": []string{getUserAgent()}}),
	)
	return err
}

// Disconnect closes the connection to the OBS WebSocket server
func Disconnect() error {
	if Client != nil {
		return Client.Disconnect()
	}
	return nil
}

func getUserAgent() string {
	userAgent := "obs-cli"
	if Version != "" {
		userAgent += "/" + Version
	}
	return userAgent
}
