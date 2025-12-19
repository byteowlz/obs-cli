package outputs

import (
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	recordingCmd = &coral.Command{
		Use:   "recording",
		Short: "manage recordings",
		Long:  `The recording command manages recordings`,
		RunE:  nil,
	}

	startStopRecordingCmd = &coral.Command{
		Use:   "toggle",
		Short: "Toggle recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return toggleRecording()
		},
	}

	startRecordingCmd = &coral.Command{
		Use:   "start",
		Short: "Starts recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return startRecording()
		},
	}

	stopRecordingCmd = &coral.Command{
		Use:   "stop",
		Short: "Stops recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return stopRecording()
		},
	}

	pauseRecordingCmd = &coral.Command{
		Use:   "pause",
		Short: "manage paused state",
	}

	enablePauseRecordingCmd = &coral.Command{
		Use:   "enable",
		Short: "Pause recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return pauseRecording()
		},
	}

	resumePauseRecordingCmd = &coral.Command{
		Use:   "resume",
		Short: "Resume recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return resumeRecording()
		},
	}

	togglePauseRecordingCmd = &coral.Command{
		Use:   "toggle",
		Short: "Pause/resume recording",
		RunE: func(cmd *coral.Command, args []string) error {
			return togglePauseRecording()
		},
	}

	recordingStatusCmd = &coral.Command{
		Use:   "status",
		Short: "Reports recording status",
		RunE: func(cmd *coral.Command, args []string) error {
			return recordingStatus()
		},
	}
)

func toggleRecording() error {
	_, err := client.Client.Record.ToggleRecord()
	return err
}

func startRecording() error {
	_, err := client.Client.Record.StartRecord()
	return err
}

func stopRecording() error {
	_, err := client.Client.Record.StopRecord()
	return err
}

func pauseRecording() error {
	_, err := client.Client.Record.PauseRecord()
	return err
}

func resumeRecording() error {
	_, err := client.Client.Record.ResumeRecord()
	return err
}

func togglePauseRecording() error {
	_, err := client.Client.Record.ToggleRecordPause()
	return err
}

func recordingStatus() error {
	r, err := client.Client.Record.GetRecordStatus()
	if err != nil {
		return err
	}

	fmt.Printf("Recording: %s\n", strconv.FormatBool(r.OutputActive))
	if !r.OutputActive {
		return nil
	}

	fmt.Printf("Paused: %s\n", strconv.FormatBool(r.OutputPaused))
	fmt.Printf("Timecode: %s\n", r.OutputTimecode)
	fmt.Printf("Duration: %.0f ms\n", r.OutputDuration)
	fmt.Printf("Bytes: %.0f\n", r.OutputBytes)

	// Try to get file size if path is available
	if r.OutputBytes > 0 {
		fmt.Printf("Size: %s\n", humanize.Bytes(uint64(r.OutputBytes)))
	}

	return nil
}

func init() {
	pauseRecordingCmd.AddCommand(enablePauseRecordingCmd)
	pauseRecordingCmd.AddCommand(resumePauseRecordingCmd)
	pauseRecordingCmd.AddCommand(togglePauseRecordingCmd)

	recordingCmd.AddCommand(startStopRecordingCmd)
	recordingCmd.AddCommand(startRecordingCmd)
	recordingCmd.AddCommand(stopRecordingCmd)
	recordingCmd.AddCommand(pauseRecordingCmd)
	recordingCmd.AddCommand(recordingStatusCmd)
}

// RegisterRecordingCommands adds all recording commands to the given parent command
func RegisterRecordingCommands(parent *coral.Command) {
	parent.AddCommand(recordingCmd)
}
