package outputs

import (
	"fmt"
	"strconv"

	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	streamCmd = &coral.Command{
		Use:   "stream",
		Short: "manage streams",
		Long:  `The stream command manages streams`,
		RunE:  nil,
	}

	startStopStreamCmd = &coral.Command{
		Use:   "toggle",
		Short: "Toggle streaming",
		RunE: func(cmd *coral.Command, args []string) error {
			return toggleStream()
		},
	}

	startStreamCmd = &coral.Command{
		Use:   "start",
		Short: "Starts streaming",
		RunE: func(cmd *coral.Command, args []string) error {
			return startStream()
		},
	}

	stopStreamCmd = &coral.Command{
		Use:   "stop",
		Short: "Stops streaming",
		RunE: func(cmd *coral.Command, args []string) error {
			return stopStream()
		},
	}

	streamStatusCmd = &coral.Command{
		Use:   "status",
		Short: "Reports streaming status",
		RunE: func(cmd *coral.Command, args []string) error {
			return streamStatus()
		},
	}
)

func toggleStream() error {
	_, err := client.Client.Stream.ToggleStream()
	return err
}

func startStream() error {
	_, err := client.Client.Stream.StartStream()
	return err
}

func stopStream() error {
	_, err := client.Client.Stream.StopStream()
	return err
}

func streamStatus() error {
	r, err := client.Client.Stream.GetStreamStatus()
	if err != nil {
		return err
	}

	fmt.Printf("Streaming: %s\n", strconv.FormatBool(r.OutputActive))
	if !r.OutputActive {
		return nil
	}

	fmt.Printf("Timecode: %s\n", r.OutputTimecode)
	fmt.Printf("Duration: %.0f ms\n", r.OutputDuration)
	fmt.Printf("Bytes: %.0f\n", r.OutputBytes)
	fmt.Printf("Skipped Frames: %.0f\n", r.OutputSkippedFrames)
	fmt.Printf("Total Frames: %.0f\n", r.OutputTotalFrames)

	return nil
}

func init() {
	streamCmd.AddCommand(startStopStreamCmd)
	streamCmd.AddCommand(startStreamCmd)
	streamCmd.AddCommand(stopStreamCmd)
	streamCmd.AddCommand(streamStatusCmd)
}

// RegisterCommands adds all stream commands to the given parent command
func RegisterStreamCommands(parent *coral.Command) {
	parent.AddCommand(streamCmd)
}
