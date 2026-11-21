#!/bin/sh
set -eu

# Keep the release tag and checksum in sync with the published GitHub asset.
version=v1.3.0
checksum=d9f38969a1ea4c8c2401165f232000aad6a4f9da47cc907d6fc89c95d61eed0a
asset=herdr-sync-panes_linux_amd64.tar.gz
url="https://github.com/psauxwwf/herdr-sync-panes/releases/download/$version/$asset"

if [ "$(uname -s)" != Linux ]; then
	echo "herdr-sync-panes: release installation supports Linux only" >&2
	exit 1
fi
case "$(uname -m)" in
x86_64 | amd64) ;;
*)
	echo "herdr-sync-panes: no release binary for this architecture (requires amd64)" >&2
	exit 1
	;;
esac

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/herdr-sync-panes.XXXXXXXX")
trap 'rm -rf "$tmpdir"' EXIT
trap 'exit 1' HUP INT TERM
archive=$tmpdir/$asset
binary=$tmpdir/herdr-sync-panes

curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 \
	--output "$archive" "$url"
printf '%s  %s\n' "$checksum" "$archive" | sha256sum --check --status || {
	echo "herdr-sync-panes: release checksum mismatch" >&2
	exit 1
}
tar -xOf "$archive" herdr-sync-panes >"$binary"
chmod 755 "$binary"
mkdir -p bin
mv "$binary" bin/herdr-sync-panes
echo "Installed herdr-sync-panes $version (linux/amd64)"
