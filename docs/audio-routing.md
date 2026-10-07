# Stable isolated audio with Source Record

Use OBS's synchronized global audio mixes for the source recordings, not Source Record's per-source/custom audio-copy callback. A local Source Record 0.4.8 test reproduced discontinuities on a clean tone through the custom path; switching only the audio route to global mixes removed them. The AAC encoder and video settings stayed unchanged.

## Recommended three-output setup

In OBS **Advanced Audio Properties**:

| Input | Enabled audio tracks |
|---|---|
| Desktop capture | 1 (mix), 2 (desktop only) |
| Microphone input | 1 (mix), 3 (mic only) |
| Camera/HDMI audio | Muted; no audio tracks |

Main recording: enable tracks **1, 2, 3**. Track 1 is the complete mix; 2 and 3 are editing/recovery stems. Do not play all three together in an editor, or the audio will be doubled.

In each Source Record filter, enable **Different Audio**:

- Desktop: choose **Track 2**; leave the Source selector empty.
- Camera: choose **Track 3**; leave the Source selector empty.

Equivalent filter settings:

```json
{"different_audio": true, "audio_track": 2, "audio_source": ""}
```

Use `audio_track: 3` for the camera/microphone output. A positive numbered track reads the global OBS mix. “None” (`audio_track: 0`) plus a Source name still uses the custom copy callback; it does not provide the synchronized mix path. “All” (`-1`) also uses the global mix but records multiple tracks rather than a single isolated stem.

This preserves desktop-only sound in the desktop file and mic-only sound in the camera file. It does not change encoders, sample rates, video resolution, or composition. The CLI warns about custom routes on stderr but does not silently change user-owned audio routing.

Only change routing while recording and streaming are stopped. Verify the producer checkboxes: selecting a numbered track alone does not make that track contain the intended input.

## Recover audio from an existing program recording

For this three-track setup, extract the clean stems without re-encoding:

```sh
ffmpeg -n -i program.mkv -map 0:a:1 -vn -c:a copy desktop-clean.m4a
ffmpeg -n -i program.mkv -map 0:a:2 -vn -c:a copy mic-clean.m4a
```

Audio stream indexes are zero-based: `0:a:0` is the full mix. Confirm the stream layout first if using a different setup. Import the clean stems alongside the isolated videos and mute their damaged original audio. Separate outputs can have slightly different start times; align waveforms or a slate before final export. Keep the originals.

## Evidence and limits

A six-second steady region of a 997 Hz synthetic tone was compared against the corresponding program track. Custom copying produced maximum sample discontinuities about ten times the reference and correlations of roughly 0.70–0.81. Global-mix routing produced correlation above 0.9999, with sample steps matching the clean reference. Both routes used the same FFmpeg AAC encoder, ruling out the encoder alone for this reproduction.

The test used temporary media/color sources because the physical camera was disconnected; it validates the audio path, not camera hardware. No private recordings or raw audio are committed here. Real affected clips still need listening/verification, and devices, plugins, or playback applications can have other independent faults.
