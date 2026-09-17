#!/usr/bin/env bash
#
# Install the meta-ads binary so it runs from anywhere (no ./ prefix).
#
# Three execution modes:
#
#   1. From a checkout of the repo (reuses the prebuilt ./meta-ads if present,
#      otherwise builds it — requires a Go toolchain):
#        ./install.sh [DEST_DIR]
#
#   2. Piped from GitHub with a prebuilt release binary available (no Go needed):
#        curl -fsSL https://raw.githubusercontent.com/rizbud/meta-ads-cli/main/install.sh | bash
#
#   3. Piped from GitHub with no matching release binary for this OS/arch yet
#      (downloads the source tarball and builds it — requires a Go toolchain).
#
# Install directory selection (argument wins over INSTALL_DIR):
#   1. First positional argument:     ./install.sh /custom/bin
#   2. INSTALL_DIR env var:           INSTALL_DIR=/custom/bin ./install.sh
#   3. Auto-detect: the first existing bin directory already on PATH
#      (~/go/bin, ~/.local/bin, /usr/local/bin), falling back to ~/.local/bin.
#
# Prebuilt binaries live on GitHub Releases as meta-ads-<os>-<arch> (e.g.
# meta-ads-linux-amd64, meta-ads-darwin-arm64) and are auto-detected from the
# host. Override with GOOS/GOARCH env vars.

set -euo pipefail

OWNER="rizbud"
REPO="meta-ads-cli"
BRANCH="main"

# Path of the binary to install; set by resolve_binary().
binary=""

# Returns 0 if directory $1 is already on PATH.
on_path() {
  case ":$PATH:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

require_go() {
  if ! command -v go >/dev/null 2>&1; then
    echo "error: this install path requires Go 1.23+ on your PATH." >&2
    echo "       Either install Go and re-run, or rely on a prebuilt release binary." >&2
    exit 1
  fi
}

# Echo the release asset name for the current platform, e.g. meta-ads-linux-amd64.
asset_name() {
  local os arch
  os="${GOOS:-}"
  if [[ -z "$os" ]]; then
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  fi
  case "$os" in
    linux | darwin) ;;
    *) echo "error: unsupported or unknown OS: $os" >&2; return 1 ;;
  esac

  arch="${GOARCH:-}"
  if [[ -z "$arch" ]]; then
    arch="$(uname -m)"
  fi
  case "$arch" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) echo "error: unsupported or unknown architecture: $arch" >&2; return 1 ;;
  esac

  echo "meta-ads-$os-$arch"
}

# Echo the first existing bin directory that is already on PATH,
# else fall back to the standard user bin dir.
pick_dir() {
  local d
  for d in \
    "$(go env GOBIN 2>/dev/null || true)" \
    "$(go env GOPATH 2>/dev/null || true)/bin" \
    "$HOME/.local/bin" \
    "/usr/local/bin"
  do
    [[ -z "$d" || ! -d "$d" ]] && continue
    if on_path "$d"; then
      echo "$d"
      return
    fi
  done
  echo "$HOME/.local/bin"
}

# Resolve and prepare the binary to install. Sets the global $binary.
resolve_binary() {
  local script repo
  script="${BASH_SOURCE[0]:-}"

  # Mode 1: running from a repo checkout.
  if [[ -n "$script" ]]; then
    repo="$(cd "$(dirname "$script")" 2>/dev/null && pwd || true)"
    if [[ -n "$repo" && -f "$repo/go.mod" ]]; then
      binary="$repo/meta-ads"
      if [[ ! -x "$binary" ]]; then
        require_go
        echo ">> Building meta-ads ..."
        (cd "$repo" && go build -o meta-ads .)
      fi
      return
    fi
  fi

  # Modes 2 & 3: piped from GitHub.
  BUILD_DIR="$(mktemp -d)"
  trap 'rm -rf "$BUILD_DIR"' EXIT

  # Mode 2: try a prebuilt release binary first — no Go needed.
  if name="$(asset_name)" && \
     curl -fsSL "https://github.com/$OWNER/$REPO/releases/latest/download/$name" \
       -o "$BUILD_DIR/$name" 2>/dev/null; then
    chmod +x "$BUILD_DIR/$name"
    binary="$BUILD_DIR/$name"
    echo ">> Using prebuilt $name"
    return
  fi

  # Mode 3: no matching prebuilt binary yet — build from the source tarball.
  require_go
  echo ">> No prebuilt binary available; building from source ..."
  curl -fsSL "https://codeload.github.com/$OWNER/$REPO/tar.gz/refs/heads/$BRANCH" \
    -o "$BUILD_DIR/src.tar.gz" || {
    echo "error: could not download $OWNER/$REPO from GitHub." >&2
    exit 1
  }
  tar -xzf "$BUILD_DIR/src.tar.gz" -C "$BUILD_DIR"
  binary="$BUILD_DIR/$REPO-$BRANCH/meta-ads"
  (cd "$BUILD_DIR/$REPO-$BRANCH" && go build -o meta-ads .)
}

# Install $binary into $1 (creating it if needed, honoring read-only dirs).
install_binary() {
  local dest="$1"
  echo ">> Installing meta-ads to $dest"
  mkdir -p "$dest"
  if [[ ! -w "$dest" ]]; then
    if command -v sudo >/dev/null 2>&1; then
      sudo install -m 0755 "$binary" "$dest/meta-ads"
    else
      echo "error: no write permission on $dest (and no sudo available)" >&2
      exit 1
    fi
  else
    install -m 0755 "$binary" "$dest/meta-ads"
  fi
}

# Confirm the install and help the user run the command.
verify() {
  local dest="$1"
  "$dest/meta-ads" --version
  if on_path "$dest"; then
    echo ">> Done. Run it from anywhere with:  meta-ads --help"
  else
    echo ">> Installed $dest/meta-ads, but $dest is not on your PATH."
    echo "   Add it now (append to ~/.bashrc to persist):"
    echo "     export PATH=\"$dest:\$PATH\""
  fi
}

main() {
  local install_dir

  resolve_binary

  install_dir=""
  if [[ $# -ge 1 ]]; then
    install_dir="$1"
  elif [[ -n "${INSTALL_DIR:-}" ]]; then
    install_dir="$INSTALL_DIR"
  else
    install_dir="$(pick_dir)"
  fi

  install_binary "$install_dir"
  verify "$install_dir"
}

main "$@"