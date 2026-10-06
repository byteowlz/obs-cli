#!/usr/bin/env bash
set -euo pipefail
source_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$source_dir/../.." && pwd)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/obs-cli-launcher-test.XXXXXX")
trap 'rm -rf -- "$workdir"' EXIT
app="$workdir/space in path/OBS Recording.app"
cli="$root/dist/obs-cli"
version=$(tr -d '\r\n' < "$root/VERSION")

# Version and help must work without contacting OBS or reading credentials.
[[ $(OBS_CLI_HOST=127.0.0.1 OBS_CLI_PORT=9 "$cli" --version) == "obs-cli version $version" ]]
OBS_CLI_HOST=127.0.0.1 OBS_CLI_PORT=9 "$cli" --help >/dev/null
"$source_dir/build.sh" --help >/dev/null
if "$source_dir/build.sh" --cli "$workdir/missing" --output "$app" >/dev/null 2>&1; then exit 1; fi
[[ ! -e "$app" ]]
if "$source_dir/build.sh" --cli "$cli" --output "$workdir/not-an-app" >/dev/null 2>&1; then exit 1; fi
[[ ! -e "$workdir/not-an-app" ]]

"$source_dir/build.sh" --cli "$cli" --output "$app" >/dev/null
[[ -f "$app/Contents/Resources/Scripts/main.scpt" ]]
[[ $("$app/Contents/Resources/obs-cli" --version) == "obs-cli version $version" ]]
[[ $(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist") == "$version" ]]
codesign --verify --deep --strict "$app"
"$app/Contents/Resources/obs-recording-displays" > "$workdir/displays.tsv"
rows=0
while IFS=$'\t' read -r uuid label; do
    [[ -n "$uuid" ]] || continue # Headless macOS CI may have no active displays.
    [[ "$uuid" =~ ^[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}$ ]]
    [[ -n "$label" ]]
    rows=$((rows + 1))
done < "$workdir/displays.tsv"

printf 'keep\n' > "$app/sentinel"
if "$source_dir/build.sh" --cli "$cli" --output "$app" >/dev/null 2>&1; then exit 1; fi
[[ $(< "$app/sentinel") == keep ]]
"$source_dir/build.sh" --cli "$cli" --output "$app" --force >/dev/null
[[ ! -e "$app/sentinel" ]]
[[ $("$app/Contents/Resources/obs-cli" --version) == "obs-cli version $version" ]]
link="$workdir/link.app"
ln -s "$app" "$link"
if "$source_dir/build.sh" --cli "$cli" --output "$link" --force >/dev/null 2>&1; then exit 1; fi
[[ -L "$link" && -f "$app/Contents/Resources/Scripts/main.scpt" ]]
printf 'macOS launcher checks passed (active display rows: %s)\n' "$rows"
