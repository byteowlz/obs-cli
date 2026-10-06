# obs-cli

[![Build Status](https://github.com/byteowlz/obs-cli/actions/workflows/build.yml/badge.svg)](https://github.com/byteowlz/obs-cli/actions)
[![Go ReportCard](https://goreportcard.com/badge/muesli/obs-cli)](https://goreportcard.com/report/muesli/obs-cli)
[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://pkg.go.dev/github.com/muesli/obs-cli)

This byteowlz fork of [muesli/obs-cli](https://github.com/muesli/obs-cli) remotely controls OBS through its WebSocket server (built into current OBS versions).

## Installation

### Packages & Binaries

Upstream/AUR packages do not include this fork's named-session additions. Build from this repository, or use its own release artifacts when published.

### Build From Source

Alternatively you can also build `obs-cli` from source. Make sure you have a
working Go environment (Go 1.23 or higher is required). See the
[install instructions](https://golang.org/doc/install.html).

```sh
git clone https://github.com/byteowlz/obs-cli.git
cd obs-cli
just install
obs-cli --version
```

### Optional macOS launcher

```sh
just install-macos-app
```

This generates a self-contained `~/Applications/OBS Recording.app` with a screen/name picker. Maintained AppleScript, Swift helper, and build/install sources live in [`integrations/macos/`](integrations/macos/README.md); generated app bundles are not committed. Config, credentials, state, and recordings stay outside git. OBS recording sources and the CLI connection must already be configured.

### Versioning

`VERSION` is the local build/install version. GoReleaser injects its release version into the same `main.version` field. `just release-check` validates code and archive configuration; tagged releases use `v<version>`. CI builds snapshot archives without publishing a GitHub release. See [CHANGELOG.md](CHANGELOG.md).

## Usage

All commands support the following flags:

- `--host`: which OBS instance to connect to
- `--port`: port to connect to
- `--password`: password used for authentication

### Streams

Change the streaming state:

```
obs-cli stream start
obs-cli stream stop
obs-cli stream toggle
```

Display streaming status:

```
obs-cli stream status
```

### Recordings

Change the recording state:

```
obs-cli recording start
obs-cli recording stop
obs-cli recording toggle
```

Pause or resume a recording:

```
obs-cli recording pause enable
obs-cli recording pause resume
obs-cli recording pause toggle
```

Display recording status:

```
obs-cli recording status
```

#### Named recording sessions

```sh
obs-cli recording session "Take one"
# Optional macOS display selection; obtain UUID from an OS display picker.
obs-cli recording session "Take two" --screen <display-UUID> --composition screen
obs-cli recording session "Fixed canvas" --composition 16:9
obs-cli recording session "Existing layout" --composition keep
# Or omit the name to prompt on interactive stdin (not in CI).
obs-cli recording session
```

On success, stdout is the absolute directory created under
`~/Movies/YYYY-MM-DD_name/`. Repeated names get `_2`, `_3`, etc.; existing
folders, files and symlinks are never reused (up to 1000 attempts). Names are
1-80 bytes: letters, numbers, spaces, dots, hyphens and underscores; no paths,
surrounding whitespace, `.` or `..`.

This is a **local recording workflow**: run the CLI on the OBS computer so the
reserved directory and OBS output paths refer to the same filesystem. OBS must
already have:

- Collection and recording profile `MultiTrack`, with program scene `Composite`.
- Sources `Desktop` and `Cam Link`, each with an enabled `Source Record` filter
  of kind `source_record_filter`, configured to record when main recording starts.
- Main MKV output with filename formatting `program`. Both source filters must
  use `rec_format=mkv` and `record_mode=3` (follow main recording). Timestamped
  source filenames such as `desktop-%hh-%mm-%ss` and `camlink-%hh-%mm-%ss` are
  recommended because Source Record can overwrite fixed names on manual restarts.
  The CLI validates format and recording mode plus the main filename template;
  it changes none of these settings. Encoder compatibility and actual isolated
  outputs still need testing.

Active recording (including paused recording), active streaming, mismatched
bindings, missing/disabled/wrong filters and unreadable paths are rejected before
any changes. All old paths and both valid source filters are checked first.
Composition is prepared before reserving the folder. The program directory and
**both** filter `path` settings are then applied and read back before `StartRecord`.
Path updates overlay only `path`, preserving filenames and all other settings.

Composition defaults to `screen`: the canvas follows the **actual native Desktop
capture size**, not its transformed scene rectangle or OS logical display size.
Canvas/output dimensions are even; the output is aspect-preserving, rounded to the
nearest even pixels and capped at `output_max_dimension` (default 1920), without
upscaling small captures. For example, 3600x2338 produces canvas 3600x2338 and
output 1920x1246. Odd native canvas dimensions round down to even; Desktop fits
centered with no crop. `16:9` uses a fixed 1920x1080 canvas/output (reduced by a
smaller configured cap) and letterboxes Desktop. The camera is a quarter of the
shown Desktop width, preserves its native aspect, and sits bottom-right **inside
Desktop content**, inset 1.25% of its displayed width, never in a black bar. Extreme
tall cameras/panoramas reduce the PIP/inset further to keep it inside the content.
`keep` leaves existing video settings and transforms untouched.

`--composition screen|16:9|keep` overrides config. Optional `--screen UUID` overlays
only `display_uuid` and `type=0` on the configured Desktop source; it requires a
macOS `screen_capture` input. Without this flag, the Desktop Properties choice is
preserved and input settings are not queried. Display enumeration/selection UI
belongs to an external OS picker; this command never queries OBS display-property
lists. Desktop must have nonzero native dimensions stable across two successive
reads (100ms intervals, at most 51 reads); explicit selection must also match the
requested UUID/type. Unknown, zero or oversized dimensions fail before start.
Source names are ordered **[desktop, camera]**; both must be direct items in the
configured program scene for adaptive composition.

When base/output resolutions change, Source Record private views must not survive
an OBS video reset. The adapter snapshots **both complete filters** (all settings,
index, enabled state), removes both, waits 500ms for queued graphics cleanup,
applies video settings/transforms, recreates/verifies both identical filters, and
waits 500ms for recreated views to settle. Idle status/bindings are rechecked before
mutations. A reset/restoration failure attempts to restore every removed filter
best-effort and **never starts recording**. If video resolutions already match,
there is no reset or filter removal/recreation. Native isolated source resolutions,
encoder/audio/filename settings and FPS are preserved. Selection/layout changes
may remain applied on pre-start failure; only session path changes are rolled back.

Before requesting start, failures restore attempted path changes in reverse order,
best effort, and report rollback errors. Once start is requested, output paths and
reserved folders are retained even on error: OBS may still be initializing or
recording. Success waits up to five seconds (100ms polling) for the main recording
to become active, rather than treating the asynchronous StartRecord acknowledgement
as activation. On any uncertain start, check `recording status` before retrying.
Requests use the existing WebSocket timeout; folder collision retries are bounded.
OBS has no atomic session transaction: do not run concurrent session commands or change OBS during
startup. This command never switches collections/profiles/scenes or changes camera
hardware; optional `--screen` changes only the Desktop display selection.

Use `recording stop` and `recording status` as usual. Pause propagation depends
on the Source Record version; this workflow validates start/stop, not pause/resume.
Avoid pausing until you have tested all outputs together.
Stopping the main output relies on the source filters' existing recording-mode
configuration; status reports the main output, not individual source outputs.

Add this optional table to `$XDG_CONFIG_HOME/obs-cli/config.toml` (default:
`~/.config/obs-cli/config.toml`), or any file selected with `--config`, such as
`user-config.toml`. Missing fields retain these defaults; lists replace defaults.
`obs_profile` is the OBS recording profile, not `--profile` (server connection).

```toml
[recording_session]
base_directory = "~/Movies" # ~ and environment variables expand; absolute local path
scene_collection = "MultiTrack"
obs_profile = "MultiTrack"
scene = "Composite"
source_names = ["Desktop", "Cam Link"] # ordered desktop, camera; two distinct sources
filter_name = "Source Record"          # same filter name on both sources
composition = "screen"                # screen | 16:9 | keep
output_max_dimension = 1920            # even output cap; accepted range 2..16384
```

```sh
obs-cli --config ~/.config/obs-cli/user-config.toml recording session demo
```

The local schema is [`examples/config.schema.json`](examples/config.schema.json).
For editor support in a copied config, set `#:schema` to that file's absolute path.
First-run config generation includes the session defaults without overwriting
existing configs. Developers can run `just check` (build, tests, vet).

### Scenes

List all scene names:

```
obs-cli scene list
```

Show the current scene name:

```
obs-cli scene get
```

Switch program to a scene:

```
obs-cli scene current <scene>
```

Switch preview to a scene (studio mode must be enabled):

```
obs-cli scene preview <scene>
```

Switch program (studio mode disabled) or preview (studio mode enabled) to a scene:

```
obs-cli scene switch <scene>
```

### Scene Collections

List all scene collections:

```
obs-cli scenecollection list
```

Show the current scene collection:

```
obs-cli scenecollection get
```

Switch to a scene collection:

```
obs-cli scenecollection set <scenecollection>
```

### Scene Items

List all items of a scene:

```
obs-cli sceneitem list <scene>
```

Change the visibility of a scene-item:

```
obs-cli sceneitem show <scene> <item>
obs-cli sceneitem hide <scene> <item>
obs-cli sceneitem toggle <scene> <item>
```

Display the visibility of a scene-item:

```
obs-cli sceneitem visible <scene> <item>
```

Center a scene-item horizontally:

```
obs-cli sceneitem center <scene> <item>
```

### Labels

Change a FreeType text label:

```
obs-cli label text <label> <text>
```

Trigger a countdown and continuously update a label with the remaining time:

```
obs-cli label countdown <label> <duration>
```

### Sources

List special sources:

```
obs-cli source list
```

Toggle mute status of a source:

```
obs-cli source toggle-mute <source>
```

### Studio Mode

Enable or disable Studio Mode:

```
obs-cli studiomode enable
obs-cli studiomode disable
obs-cli studiomode toggle
```

Display studio mode status:

```
obs-cli studiomode status
```

Transition to program (when the studio mode is enabled):

```
obs-cli studiomode transition
```

### Profiles

List all profiles:

```
obs-cli profile list
```

Show the current profile:

```
obs-cli profile get
```

Switch to a profile:

```
obs-cli profile set <profile>
```

### Replay Buffer

Change the replay buffer state:

```
obs-cli replaybuffer start
obs-cli replaybuffer stop
```

Save the replay buffer:

```
obs-cli replaybuffer save
```

Display replay buffer status:

```
obs-cli replaybuffer status
```

### Virtual Camera

Change the virtual camera state:

```
obs-cli virtualcam start
obs-cli virtualcam stop
obs-cli virtualcam toggle
```

Display virtual camera status:

```
obs-cli virtualcam status
```
