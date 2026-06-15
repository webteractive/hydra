#!/bin/sh
# hydra installer — downloads the latest release binary for your platform.
#
#   curl -fsSL https://raw.githubusercontent.com/webteractive/hydra/main/install.sh | sh
#
set -eu

REPO="webteractive/hydra"
INSTALL_DIR="$HOME/.local/bin"

err() { printf 'hydra-install: %s\n' "$1" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar  >/dev/null 2>&1 || err "tar is required"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin | linux) ;;
  *) err "unsupported OS '$os' — prebuilt binaries exist for darwin and linux only" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) err "unsupported architecture '$arch'" ;;
esac

# Always install the latest published release.
tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
[ -n "$tag" ] || err "could not determine the latest release tag"
ver="${tag#v}"

file="hydra_${ver}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'Downloading hydra %s (%s/%s)...\n' "$tag" "$os" "$arch"
curl -fsSL "$base/$file" -o "$tmp/$file" || err "download failed: $base/$file"

# Verify checksum when a sha256 tool is available.
if curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then sha='sha256sum'
  elif command -v shasum >/dev/null 2>&1; then sha='shasum -a 256'
  else sha=''; fi
  if [ -n "$sha" ]; then
    want=$(grep " ${file}\$" "$tmp/checksums.txt" | awk '{print $1}')
    got=$($sha "$tmp/$file" | awk '{print $1}')
    [ -n "$want" ] && [ "$want" = "$got" ] || err "checksum mismatch for $file"
    printf 'Checksum OK.\n'
  fi
fi

tar -xzf "$tmp/$file" -C "$tmp" hydra || err "failed to extract hydra from $file"
mkdir -p "$INSTALL_DIR"
mv "$tmp/hydra" "$INSTALL_DIR/hydra"
chmod +x "$INSTALL_DIR/hydra"

printf 'Installed hydra %s to %s/hydra\n' "$ver" "$INSTALL_DIR"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf '\nNote: %s is not on your PATH. Add it to your shell profile:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac
