package outputs

import (
	"fmt"
	"strconv"

	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	replayBufferCmd = &coral.Command{
		Use:   "replaybuffer",
		Short: "manage replay buffer",
		Long:  `The replaybuffer command manages the replay buffer`,
		RunE:  nil,
	}

	startReplayBufferCmd = &coral.Command{
		Use:   "start",
		Short: "Starts replay buffer",
		RunE: func(cmd *coral.Command, args []string) error {
			return startReplayBuffer()
		},
	}

	stopReplayBufferCmd = &coral.Command{
		Use:   "stop",
		Short: "Stops replay buffer",
		RunE: func(cmd *coral.Command, args []string) error {
			return stopReplayBuffer()
		},
	}

	saveReplayBufferCmd = &coral.Command{
		Use:   "save",
		Short: "Saves replay buffer",
		RunE: func(cmd *coral.Command, args []string) error {
			return saveReplayBuffer()
		},
	}

	replayBufferStatusCmd = &coral.Command{
		Use:   "status",
		Short: "Reports replay buffer status",
		RunE: func(cmd *coral.Command, args []string) error {
			return replayBufferStatus()
		},
	}

	toggleReplayBufferCmd = &coral.Command{
		Use:   "toggle",
		Short: "Toggle replay buffer",
		RunE: func(cmd *coral.Command, args []string) error {
			return toggleReplayBuffer()
		},
	}
)

func startReplayBuffer() error {
	_, err := client.Client.Outputs.StartReplayBuffer()
	return err
}

func stopReplayBuffer() error {
	_, err := client.Client.Outputs.StopReplayBuffer()
	return err
}

func saveReplayBuffer() error {
	_, err := client.Client.Outputs.SaveReplayBuffer()
	return err
}

func toggleReplayBuffer() error {
	_, err := client.Client.Outputs.ToggleReplayBuffer()
	return err
}

func replayBufferStatus() error {
	r, err := client.Client.Outputs.GetReplayBufferStatus()
	if err != nil {
		return err
	}

	fmt.Printf("Replay Buffer active: %s\n", strconv.FormatBool(r.OutputActive))
	return nil
}

func init() {
	replayBufferCmd.AddCommand(startReplayBufferCmd)
	replayBufferCmd.AddCommand(stopReplayBufferCmd)
	replayBufferCmd.AddCommand(saveReplayBufferCmd)
	replayBufferCmd.AddCommand(toggleReplayBufferCmd)
	replayBufferCmd.AddCommand(replayBufferStatusCmd)
}

// RegisterReplayBufferCommands adds all replay buffer commands to the given parent command
func RegisterReplayBufferCommands(parent *coral.Command) {
	parent.AddCommand(replayBufferCmd)
}
