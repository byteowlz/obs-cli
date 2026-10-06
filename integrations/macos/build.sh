#!/usr/bin/env bash
set -euo pipefail

source_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$source_dir/../.." && pwd)
cli="$root/dist/obs-cli"
output="$root/dist/OBS Recording.app"
version=$(tr -d '\r\n' < "$root/VERSION")
force=0

usage() {
    printf '%s\n' 'Build a self-contained macOS recording launcher.' \
        'Usage: build.sh [--cli PATH] [--output PATH.app] [--version VERSION] [--force]' \
        'Defaults: dist/obs-cli, dist/OBS Recording.app, VERSION from this source/archive.' \
        'Existing outputs are refused unless --force is explicit. Symlinks are always refused.'
}
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }
while (($#)); do
    case "$1" in
        --cli|--output|--version)
            (($# >= 2)) || fail "$1 needs a value"
            case "$1" in --cli) cli=$2 ;; --output) output=$2 ;; --version) version=$2 ;; esac
            shift 2 ;;
        --force) force=1; shift ;;
        --help|-h) usage; exit 0 ;;
        *) usage >&2; fail "unknown argument: $1" ;;
    esac
done

[[ $(uname -s) == Darwin ]] || fail 'this launcher requires macOS'
[[ "$output" == *.app && "$output" != *.app/ ]] || fail 'output must end in .app'
[[ ! -L "$output" ]] || fail 'refusing a symlink output'
if [[ -e "$output" ]]; then
    [[ -d "$output" && $force == 1 ]] || fail 'output exists; use --force to replace this app explicitly'
fi
[[ -x "$cli" ]] || fail 'CLI binary is missing or not executable; run just binary or pass --cli'
base_version=${version%%[-+]*}
[[ "$base_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'version must start with a three-part semantic version'
for tool in swiftc osacompile codesign; do
    command -v "$tool" >/dev/null || fail "missing $tool (install Xcode Command Line Tools)"
done
[[ $("$cli" --version) == "obs-cli version $version" ]] || fail 'CLI version does not match; build with just binary or pass its --version'
"$cli" --help >/dev/null

parent=$(dirname -- "$output")
mkdir -p -- "$parent"
parent=$(cd -- "$parent" && pwd)
output="$parent/$(basename -- "$output")"
workdir=$(mktemp -d "$parent/.obs-cli-app.XXXXXX")
cleanup() {
    status=$?
    if [[ -d "$workdir/previous.app" && ! -e "$output" && ! -L "$output" ]]; then
        mv -- "$workdir/previous.app" "$output" || printf 'warning: prior app remains at %s\n' "$workdir/previous.app" >&2
    fi
    # Preserve the backup if restoration failed; otherwise clean only our staging directory.
    if [[ -d "$workdir/previous.app" && ! -e "$output" ]]; then return; fi
    rm -rf -- "$workdir"
    return "$status"
}
trap cleanup EXIT
app="$workdir/OBS Recording.app"
osacompile -o "$app" "$source_dir/Recording.applescript"
resources="$app/Contents/Resources"
swiftc -O "$source_dir/Displays.swift" -o "$resources/obs-recording-displays"
cp -- "$cli" "$resources/obs-cli"
chmod 755 "$resources/obs-cli" "$resources/obs-recording-displays"
plist="$app/Contents/Info.plist"
set_metadata() {
    /usr/libexec/PlistBuddy -c "Set :$1 $2" "$plist" 2>/dev/null || \
        /usr/libexec/PlistBuddy -c "Add :$1 string $2" "$plist"
}
set_metadata CFBundleIdentifier com.byteowlz.obs-cli.recording
set_metadata CFBundleShortVersionString "$base_version"
set_metadata CFBundleVersion "$base_version"
# Local ad-hoc signing only: no certificates, credentials, notarization or services.
codesign --force --sign - "$resources/obs-cli"
codesign --force --sign - "$resources/obs-recording-displays"
codesign --force --sign - "$app"
codesign --verify --deep --strict "$app"

# Recheck after compilation so a concurrent installation is never silently replaced.
[[ ! -L "$output" ]] || fail 'output became a symlink while building'
if [[ -e "$output" ]]; then
    [[ -d "$output" && $force == 1 ]] || fail 'output appeared while building; not replacing it'
    mv -- "$output" "$workdir/previous.app"
fi
if ! mv -- "$app" "$output"; then fail 'could not install the generated app'; fi
printf '%s\n' "$output"
