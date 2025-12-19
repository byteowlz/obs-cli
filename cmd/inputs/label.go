package inputs

import (
	"errors"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	labelCmd = &coral.Command{
		Use:   "label",
		Short: "manage text labels",
		Long:  `The label command manages text labels`,
		RunE:  nil,
	}

	textCmd = &coral.Command{
		Use:   "text",
		Short: "Changes a text label",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("text requires a source and the new text")
			}
			return ChangeLabel(args[0], args[1])
		},
	}
)

// ChangeLabel updates the text of a text label source
func ChangeLabel(source string, text string) error {
	// In OBS WebSocket 5.x, text sources are managed through input settings
	// The "text" property is used for both text_gdiplus and text_ft2_source
	overlay := true
	settings := map[string]any{
		"text": text,
	}

	r := inputs.SetInputSettingsParams{
		InputName:     &source,
		InputSettings: settings,
		Overlay:       &overlay,
	}

	_, err := client.Client.Inputs.SetInputSettings(&r)
	return err
}

func init() {
	labelCmd.AddCommand(textCmd)
}

// RegisterLabelCommands adds all label commands to the given parent command
func RegisterLabelCommands(parent *coral.Command) {
	parent.AddCommand(labelCmd)
}

// GetLabelCmd returns the label command for adding subcommands
func GetLabelCmd() *coral.Command {
	return labelCmd
}
