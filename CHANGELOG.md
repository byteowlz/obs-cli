# Changelog

## 0.1.1

- Warn when Source Record uses its custom audio-copy path, which can introduce crackling.
- Document verified global-mix routing for separate desktop/microphone audio and recovery from clean program stems. Audio settings remain user-owned and are not rewritten by the CLI.

## 0.1.0

First tagged byteowlz release of the obs-cli fork.

- Named recording sessions reserve unique dated directories for program and two source recordings.
- Screen selection and aspect-matched, fixed 16:9, or preserved composition modes, with bottom-right camera placement.
- Idle-only configuration, recording-activation confirmation, uncertain-start safeguards, and Source Record lifecycle recovery around canvas resets.
- Portable macOS screen/name picker sources, bundled display helper and CLI, and safe app build/install recipes.
- Version metadata, release archives including integration sources, and CI aligned with the current Go module and native launcher checks.

Based on the upstream muesli/obs-cli project; upstream licensing and attribution are retained.
