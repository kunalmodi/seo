#!/bin/sh
# Installs the latest sitecheck release into ~/.local/bin (or $PREFIX).
#   curl -fsSL https://raw.githubusercontent.com/kunalmodi/seo/main/install.sh | sh
set -eu

REPO="kunalmodi/seo"
PREFIX="${PREFIX:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin|linux) ;;
  *) echo "sitecheck: unsupported OS $os (try: go install github.com/$REPO/cmd/sitecheck@latest)" >&2; exit 1 ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "sitecheck: unsupported CPU $arch" >&2; exit 1 ;;
esac

asset="sitecheck-$os-$arch.tar.gz"
base="${SITECHECK_DOWNLOAD_BASE:-https://github.com/$REPO/releases/latest/download}"
url="$base/$asset"
tmp=$(mktemp -d)
cleanup() { [ -n "$tmp" ] && [ -d "$tmp" ] && rm -r "$tmp"; }
trap cleanup EXIT

echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/$asset"
curl -fsSL "$url.sha256" -o "$tmp/$asset.sha256"
expected=$(cut -d' ' -f1 < "$tmp/$asset.sha256")
if command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
else
  actual=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
fi
if [ "$expected" != "$actual" ]; then
  echo "sitecheck: checksum mismatch, not installing" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$PREFIX"
install -m 755 "$tmp/sitecheck" "$PREFIX/sitecheck"
echo "Installed $PREFIX/sitecheck"
case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) echo "Add $PREFIX to your PATH, e.g.: echo 'export PATH=\"$PREFIX:\$PATH\"' >> ~/.zshrc" ;;
esac
command -v uvx >/dev/null 2>&1 || echo "Optional: install uv (brew install uv) for the content and crawl checks."
