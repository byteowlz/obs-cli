package obsconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andreykaipov/goobs/api/requests/config"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	profileCmd = &coral.Command{
		Use:   "profile",
		Short: "manage profiles",
		Long:  `The profile command manages profiles`,
		RunE:  nil,
	}

	listProfileCmd = &coral.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *coral.Command, args []string) error {
			return listProfiles()
		},
	}

	getProfileCmd = &coral.Command{
		Use:   "get",
		Short: "Get the current profile",
		RunE: func(cmd *coral.Command, args []string) error {
			return getProfile()
		},
	}

	setProfileCmd = &coral.Command{
		Use:   "set",
		Short: "Set the current profile",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("set requires a profile name as argument")
			}
			return setProfile(strings.Join(args, " "))
		},
	}
)

func listProfiles() error {
	r, err := client.Client.Config.GetProfileList()
	if err != nil {
		return err
	}

	for _, v := range r.Profiles {
		fmt.Println(v)
	}
	return nil
}

func setProfile(profileName string) error {
	r := config.SetCurrentProfileParams{
		ProfileName: &profileName,
	}
	_, err := client.Client.Config.SetCurrentProfile(&r)
	return err
}

func getProfile() error {
	r, err := client.Client.Config.GetProfileList()
	if err != nil {
		return err
	}

	fmt.Println(r.CurrentProfileName)
	return nil
}

func init() {
	profileCmd.AddCommand(listProfileCmd)
	profileCmd.AddCommand(setProfileCmd)
	profileCmd.AddCommand(getProfileCmd)
}

// RegisterProfileCommands adds all profile commands to the given parent command
func RegisterProfileCommands(parent *coral.Command) {
	parent.AddCommand(profileCmd)
}
