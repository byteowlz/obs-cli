package inputs

import (
	"errors"
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	sourceCmd = &coral.Command{
		Use:   "source",
		Short: "manage sources",
		Long:  `The source command manages sources`,
		RunE:  nil,
	}

	listSourcesCmd = &coral.Command{
		Use:   "list",
		Short: "Lists all sources",
		RunE: func(cmd *coral.Command, args []string) error {
			return listSources()
		},
	}

	toggleMuteCmd = &coral.Command{
		Use:   "toggle-mute",
		Short: "Toggles mute",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("toggle-mute requires a source name as argument")
			}
			return toggleMute(args[0])
		},
	}

	listInputsCmd = &coral.Command{
		Use:   "inputs",
		Short: "Lists all inputs",
		RunE: func(cmd *coral.Command, args []string) error {
			return listInputs()
		},
	}
)

func listSources() error {
	// In OBS WebSocket 5.x, sources are now called "inputs"
	// GetSpecialInputs returns the special audio inputs
	resp, err := client.Client.Inputs.GetSpecialInputs()
	if err != nil {
		return err
	}

	fmt.Println("Special Inputs")
	fmt.Println("==============")
	if resp.Desktop1 != "" {
		fmt.Printf("Desktop1: %s\n", resp.Desktop1)
	}
	if resp.Desktop2 != "" {
		fmt.Printf("Desktop2: %s\n", resp.Desktop2)
	}
	if resp.Mic1 != "" {
		fmt.Printf("Mic1: %s\n", resp.Mic1)
	}
	if resp.Mic2 != "" {
		fmt.Printf("Mic2: %s\n", resp.Mic2)
	}
	if resp.Mic3 != "" {
		fmt.Printf("Mic3: %s\n", resp.Mic3)
	}
	if resp.Mic4 != "" {
		fmt.Printf("Mic4: %s\n", resp.Mic4)
	}

	return nil
}

func listInputs() error {
	resp, err := client.Client.Inputs.GetInputList(&inputs.GetInputListParams{})
	if err != nil {
		return err
	}

	fmt.Println("Inputs")
	fmt.Println("======")
	for _, input := range resp.Inputs {
		fmt.Printf("%s (%s)\n", input.InputName, input.InputKind)
	}

	return nil
}

func toggleMute(source string) error {
	p := inputs.ToggleInputMuteParams{
		InputName: &source,
	}

	_, err := client.Client.Inputs.ToggleInputMute(&p)
	return err
}

func init() {
	sourceCmd.AddCommand(listSourcesCmd)
	sourceCmd.AddCommand(listInputsCmd)
	sourceCmd.AddCommand(toggleMuteCmd)
}

// RegisterSourceCommands adds all source commands to the given parent command
func RegisterSourceCommands(parent *coral.Command) {
	parent.AddCommand(sourceCmd)
}
