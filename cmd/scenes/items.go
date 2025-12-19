package scenes

import (
	"errors"
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
)

var (
	sceneItemCmd = &coral.Command{
		Use:   "sceneitem",
		Short: "manage scene items",
		Long:  `The sceneitem command manages a scene's items`,
		RunE:  nil,
	}

	listSceneItemsCmd = &coral.Command{
		Use:   "list",
		Short: "Lists all items of a scene",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("list requires a scene")
			}
			return listSceneItems(args[0])
		},
	}

	toggleSceneItemCmd = &coral.Command{
		Use:   "toggle",
		Short: "Toggles visibility of a scene-item",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("toggle requires a scene and scene-item")
			}
			return toggleSceneItem(args[0], args[1:]...)
		},
	}

	showSceneItemCmd = &coral.Command{
		Use:   "show",
		Short: "Makes a scene-item visible",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("show requires a scene and scene-item(s)")
			}
			return setSceneItemVisible(true, args[0], args[1:]...)
		},
	}

	hideSceneItemCmd = &coral.Command{
		Use:   "hide",
		Short: "Hides a scene-item",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("hide requires a scene and scene-item(s)")
			}
			return setSceneItemVisible(false, args[0], args[1:]...)
		},
	}

	getSceneItemVisibilityCmd = &coral.Command{
		Use:   "visible",
		Short: "Show visibility status of a scene-item",
		RunE: func(cmd *coral.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("visible requires a scene and scene-item")
			}
			return getSceneItemVisibility(args[0], args[1:]...)
		},
	}
)

func listSceneItems(scene string) error {
	resp, err := client.Client.SceneItems.GetSceneItemList(&sceneitems.GetSceneItemListParams{
		SceneName: &scene,
	})
	if err != nil {
		return err
	}

	for _, item := range resp.SceneItems {
		enabled := "visible"
		if !item.SceneItemEnabled {
			enabled = "hidden"
		}
		fmt.Printf("%s (id: %d, %s)\n", item.SourceName, item.SceneItemID, enabled)
	}

	return nil
}

// getSceneItemId finds the scene item ID for a source name in a scene
func getSceneItemId(scene, sourceName string) (int, error) {
	resp, err := client.Client.SceneItems.GetSceneItemId(&sceneitems.GetSceneItemIdParams{
		SceneName:  &scene,
		SourceName: &sourceName,
	})
	if err != nil {
		return 0, err
	}
	return resp.SceneItemId, nil
}

func setSceneItemVisible(visible bool, scene string, items ...string) error {
	for _, item := range items {
		itemId, err := getSceneItemId(scene, item)
		if err != nil {
			return fmt.Errorf("failed to find item %q: %w", item, err)
		}

		_, err = client.Client.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
			SceneName:        &scene,
			SceneItemId:      &itemId,
			SceneItemEnabled: &visible,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func toggleSceneItem(scene string, items ...string) error {
	for _, item := range items {
		itemId, err := getSceneItemId(scene, item)
		if err != nil {
			return fmt.Errorf("failed to find item %q: %w", item, err)
		}

		// Get current enabled state
		resp, err := client.Client.SceneItems.GetSceneItemEnabled(&sceneitems.GetSceneItemEnabledParams{
			SceneName:   &scene,
			SceneItemId: &itemId,
		})
		if err != nil {
			return err
		}

		// Toggle the state
		newEnabled := !resp.SceneItemEnabled
		_, err = client.Client.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
			SceneName:        &scene,
			SceneItemId:      &itemId,
			SceneItemEnabled: &newEnabled,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func getSceneItemVisibility(scene string, items ...string) error {
	for _, item := range items {
		itemId, err := getSceneItemId(scene, item)
		if err != nil {
			return fmt.Errorf("failed to find item %q: %w", item, err)
		}

		resp, err := client.Client.SceneItems.GetSceneItemEnabled(&sceneitems.GetSceneItemEnabledParams{
			SceneName:   &scene,
			SceneItemId: &itemId,
		})
		if err != nil {
			return err
		}

		fmt.Printf("%s: %t\n", item, resp.SceneItemEnabled)
	}

	return nil
}

func init() {
	sceneItemCmd.AddCommand(toggleSceneItemCmd)
	sceneItemCmd.AddCommand(showSceneItemCmd)
	sceneItemCmd.AddCommand(hideSceneItemCmd)
	sceneItemCmd.AddCommand(getSceneItemVisibilityCmd)
	sceneItemCmd.AddCommand(listSceneItemsCmd)
}

// RegisterSceneItemCommands adds all scene item commands to the given parent command
func RegisterSceneItemCommands(parent *coral.Command) {
	parent.AddCommand(sceneItemCmd)
}
