package ui

import (
	"fmt"
	"strconv"

	"github.com/andreykaipov/goobs/api/requests/ui"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	studioModeCmd = &coral.Command{
		Use:   "studiomode",
		Short: "manage studio mode",
		Long:  `The studiomode command manages the studio mode`,
		RunE:  nil,
	}

	disableStudioModeCmd = &coral.Command{
		Use:   "disable",
		Short: "Disables the studio mode",
		RunE: func(cmd *coral.Command, args []string) error {
			return disableStudioMode()
		},
	}

	enableStudioModeCmd = &coral.Command{
		Use:   "enable",
		Short: "Enables the studio mode",
		RunE: func(cmd *coral.Command, args []string) error {
			return enableStudioMode()
		},
	}

	studioModeStatusCmd = &coral.Command{
		Use:   "status",
		Short: "Reports studio mode status",
		RunE: func(cmd *coral.Command, args []string) error {
			return studioModeStatus()
		},
	}

	toggleStudioModeCmd = &coral.Command{
		Use:   "toggle",
		Short: "Toggles the studio mode (enable/disable)",
		RunE: func(cmd *coral.Command, args []string) error {
			return toggleStudioMode()
		},
	}

	transitionToProgramCmd = &coral.Command{
		Use:   "transition",
		Short: "Transition to program",
		RunE: func(cmd *coral.Command, args []string) error {
			return transitionToProgram()
		},
	}
)

func disableStudioMode() error {
	enabled := false
	_, err := client.Client.Ui.SetStudioModeEnabled(&ui.SetStudioModeEnabledParams{
		StudioModeEnabled: &enabled,
	})
	return err
}

func enableStudioMode() error {
	enabled := true
	_, err := client.Client.Ui.SetStudioModeEnabled(&ui.SetStudioModeEnabledParams{
		StudioModeEnabled: &enabled,
	})
	return err
}

// IsStudioModeEnabled determines if the studio mode is currently enabled in OBS.
func IsStudioModeEnabled() (bool, error) {
	r, err := client.Client.Ui.GetStudioModeEnabled()
	if err != nil {
		return false, err
	}
	return r.StudioModeEnabled, nil
}

func studioModeStatus() error {
	isStudioModeEnabled, err := IsStudioModeEnabled()
	if err != nil {
		return err
	}

	fmt.Printf("Studio Mode: %s\n", strconv.FormatBool(isStudioModeEnabled))
	return nil
}

func toggleStudioMode() error {
	enabled, err := IsStudioModeEnabled()
	if err != nil {
		return err
	}
	newEnabled := !enabled
	_, err = client.Client.Ui.SetStudioModeEnabled(&ui.SetStudioModeEnabledParams{
		StudioModeEnabled: &newEnabled,
	})
	return err
}

func transitionToProgram() error {
	// In OBS WebSocket 5.x, triggering a transition is done via TriggerStudioModeTransition
	// but this is in the transitions package. For now, we can trigger using the current transition.
	_, err := client.Client.Transitions.TriggerStudioModeTransition()
	return err
}

func init() {
	studioModeCmd.AddCommand(disableStudioModeCmd)
	studioModeCmd.AddCommand(enableStudioModeCmd)
	studioModeCmd.AddCommand(studioModeStatusCmd)
	studioModeCmd.AddCommand(toggleStudioModeCmd)
	studioModeCmd.AddCommand(transitionToProgramCmd)
}

// RegisterStudioModeCommands adds all studio mode commands to the given parent command
func RegisterStudioModeCommands(parent *coral.Command) {
	parent.AddCommand(studioModeCmd)
}
