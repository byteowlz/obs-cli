# macOS recording launcher

Optional native screen/name picker around `obs-cli recording session`. The CLI remains the primary interface.

## Build and install

From the repository, with Go, `just`, and Xcode Command Line Tools installed:

```sh
just macos-app          # generates dist/OBS Recording.app
just install-macos-app  # explicitly replaces ~/Applications/OBS Recording.app
```

The app bundles the versioned CLI and compiled Swift display helper in `Contents/Resources/`. It has no dependency on a checkout, a particular home directory, `~/Movies`, or an executable search path at runtime. Moving the generated app is safe.

To build from an unpacked CLI release archive (no Go compiler needed):

```sh
./integrations/macos/build.sh --cli ./obs-cli --output "$HOME/Applications/OBS Recording.app"
```

Existing app directories require explicit `--force`; symlink outputs are always refused. Build happens in a sibling staging directory. Failed replacement attempts restore the previous app where possible. `--version` must match the supplied CLI; prerelease suffixes are supported, with the numeric version used in app metadata.

The build uses local ad-hoc signing, not a developer certificate or notarization. It neither installs services nor changes macOS audio routing. A downloaded app may require the usual macOS permission to open an unnotarized application.

## Use

Configure OBS and the CLI connection as described in the repository's named-session documentation. Defaults expect the `MultiTrack` profile/collection, `Composite` scene, and two enabled Source Record filters.

Open the app, select an active display, enter a name, and start. Reopen it to stop the main recording and its following source outputs. Screen choice is remembered for the next launch. The app starts OBS if needed and waits briefly for its server, but does not create or switch recording profiles/collections/scenes.

Configuration and credentials are loaded by the bundled CLI from `$XDG_CONFIG_HOME/obs-cli/config.toml` or `~/.config/obs-cli/config.toml`. Launcher state is at `$XDG_STATE_HOME/obs-cli/` or `~/.local/state/obs-cli/`. Never commit these files or recordings. “Show Last Folder” refers to the last session started by this launcher, not unrelated recordings started manually.

`Displays.swift` uses NSScreen/CoreGraphics to emit display UUIDs and native pixel dimensions. It avoids the OBS input-properties endpoint, which can crash affected macOS builds. No device identifiers are embedded in the sources.

## Validation

```sh
just macos-app-test
```

Tests compile the AppleScript and Swift helper, verify both bundled executables and signing/version metadata, exercise paths with spaces, and prove refusal of existing/symlink outputs plus explicit replacement. Headless macOS CI may have no active displays. Tests do not launch the app, contact OBS, or start recording.
