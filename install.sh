#!/bin/sh
# Nahook CLI installer.
#
# Usage:
#   curl -fsSL https://cli.nahook.com/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/getnahook/nahook-cli/main/install.sh | sh
#
# Environment overrides:
#   NAHOOK_VERSION       Pin a specific version (default: latest GitHub release).
#   NAHOOK_INSTALL_DIR   Directory to drop the binary into (default: /usr/local/bin).
#   NAHOOK_NO_COMPLETION Set to any non-empty value to skip shell-completion install.
#
# Exit codes: 0 success, non-zero on any error (network, checksum mismatch,
# unsupported platform, missing dependency).

set -eu

REPO="getnahook/nahook-cli"
BINARY="nahook"
DEFAULT_INSTALL_DIR="/usr/local/bin"

# --- helpers -----------------------------------------------------------------

# log_* writes diagnostics to stderr so stdout stays a clean "what got
# installed" trail.
log_info()  { printf '\033[1;34m==>\033[0m %s\n' "$*" >&2; }
log_ok()    { printf '\033[1;32m==>\033[0m %s\n' "$*" >&2; }
log_warn()  { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
log_error() { printf '\033[1;31m==>\033[0m %s\n' "$*" >&2; }

die() {
    log_error "$1"
    exit 1
}

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

# --- preflight ---------------------------------------------------------------

need_cmd uname
need_cmd mktemp
need_cmd tar

# curl is the network primitive; wget would work too but every modern macOS
# and most Linux distros ship curl by default. Asking the user to install
# curl is a clearer failure than maintaining a wget fallback.
need_cmd curl

# sha256 verification — try the two binary names this command goes by.
if command -v shasum >/dev/null 2>&1; then
    SHA256_CMD="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
    SHA256_CMD="sha256sum"
else
    die "missing required command: shasum or sha256sum"
fi

# --- detect OS / arch --------------------------------------------------------

uname_s=$(uname -s)
case "$uname_s" in
    Darwin) os="macos" ;;
    Linux)  os="linux" ;;
    *)      die "unsupported OS: $uname_s (only macOS and Linux are supported)" ;;
esac

uname_m=$(uname -m)
case "$uname_m" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported architecture: $uname_m" ;;
esac

# --- resolve version ---------------------------------------------------------

VERSION=${NAHOOK_VERSION:-}
if [ -z "$VERSION" ]; then
    log_info "resolving latest release of $REPO…"
    # The /releases/latest endpoint returns a JSON blob; grep+sed is enough
    # for the one field we care about and avoids a jq dependency.
    VERSION=$(curl -fsSL -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/$REPO/releases/latest" \
        | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
        | head -n1)
    if [ -z "$VERSION" ]; then
        die "could not determine latest release — set NAHOOK_VERSION to a specific tag (e.g. v0.1.0)"
    fi
fi

# goreleaser tags carry a leading v; the archive name uses the version
# without it. Strip exactly one leading v if present.
VERSION_NO_V=${VERSION#v}

INSTALL_DIR=${NAHOOK_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}

log_info "installing $BINARY $VERSION ($os/$arch) to $INSTALL_DIR"

# --- download + verify -------------------------------------------------------

ARCHIVE="${BINARY}_${VERSION_NO_V}_${os}_${arch}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
ARCHIVE_URL="$BASE_URL/$ARCHIVE"
CHECKSUMS_URL="$BASE_URL/checksums.txt"

tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t nahook-install)
trap 'rm -rf "$tmpdir"' EXIT

log_info "downloading $ARCHIVE"
if ! curl -fsSL --retry 3 --retry-delay 2 -o "$tmpdir/$ARCHIVE" "$ARCHIVE_URL"; then
    die "download failed: $ARCHIVE_URL
    Check that the release exists for $os/$arch at:
    https://github.com/$REPO/releases/tag/$VERSION"
fi

log_info "downloading checksums.txt"
curl -fsSL --retry 3 --retry-delay 2 -o "$tmpdir/checksums.txt" "$CHECKSUMS_URL" \
    || die "could not download checksums.txt from $CHECKSUMS_URL"

log_info "verifying SHA-256 checksum"
expected=$(grep " $ARCHIVE\$" "$tmpdir/checksums.txt" | awk '{print $1}')
if [ -z "$expected" ]; then
    die "no checksum entry for $ARCHIVE in checksums.txt"
fi
actual=$(cd "$tmpdir" && $SHA256_CMD "$ARCHIVE" | awk '{print $1}')
if [ "$expected" != "$actual" ]; then
    die "checksum mismatch for $ARCHIVE
    expected: $expected
    actual:   $actual"
fi

# --- extract + install -------------------------------------------------------

log_info "extracting"
tar -xzf "$tmpdir/$ARCHIVE" -C "$tmpdir" "$BINARY" \
    || die "extraction failed — archive may be corrupt"

# Move into place. If the install dir isn't writable, retry with sudo so
# the standard /usr/local/bin path works for a normal user without making
# every script-run interactive.
if [ -w "$INSTALL_DIR" ] || [ ! -e "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
    mv "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"
else
    log_warn "$INSTALL_DIR is not writable; re-running with sudo for binary install"
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"
fi
chmod +x "$INSTALL_DIR/$BINARY" 2>/dev/null || sudo chmod +x "$INSTALL_DIR/$BINARY"

# --- shell completion install ------------------------------------------------

install_completion() {
    [ -n "${NAHOOK_NO_COMPLETION:-}" ] && return 0

    user_shell=$(basename "${SHELL:-}")
    case "$user_shell" in
        bash|zsh|fish) ;;
        *)
            log_warn "shell '$user_shell' not recognised — skipping completion install (run 'nahook completion <shell>' manually)"
            return 0
            ;;
    esac

    # Generate completion via the just-installed binary. If this fails the
    # binary doesn't support `completion <shell>` for some reason — bail
    # quietly so the install still counts as a success.
    completion_script=$("$INSTALL_DIR/$BINARY" completion "$user_shell" 2>/dev/null) || {
        log_warn "could not generate $user_shell completion — skipping"
        return 0
    }

    case "$user_shell" in
        bash)
            if [ "$os" = "macos" ]; then
                target_dir="/usr/local/etc/bash_completion.d"
            else
                target_dir="/etc/bash_completion.d"
            fi
            target="$target_dir/$BINARY"
            ;;
        zsh)
            target_dir="/usr/local/share/zsh/site-functions"
            target="$target_dir/_$BINARY"
            ;;
        fish)
            target_dir="$HOME/.config/fish/completions"
            target="$target_dir/$BINARY.fish"
            ;;
    esac

    if [ -w "$(dirname "$target_dir" 2>/dev/null || echo "$target_dir")" ] \
       || [ "$user_shell" = "fish" ]; then
        mkdir -p "$target_dir"
        printf '%s\n' "$completion_script" > "$target"
    else
        log_warn "writing completion to $target requires sudo"
        sudo mkdir -p "$target_dir"
        printf '%s\n' "$completion_script" | sudo tee "$target" >/dev/null
    fi

    log_ok "installed $user_shell completion at $target"
}

install_completion

# --- done --------------------------------------------------------------------

log_ok "$BINARY $VERSION installed at $INSTALL_DIR/$BINARY"
log_info "run '$BINARY login' to authorize this device"

# Verify the install actually works by invoking --version. Failures here
# usually mean $INSTALL_DIR is not on PATH.
if ! command -v "$BINARY" >/dev/null 2>&1; then
    log_warn "$BINARY not found on PATH"
    log_warn "add $INSTALL_DIR to your PATH or invoke as $INSTALL_DIR/$BINARY"
fi
