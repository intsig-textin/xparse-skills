#!/bin/sh
set -e

# xParser CLI installer
# Usage: source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh)
#
# Environment variables:
#   XPARSER_VERSION   - version to install (default: "latest")
#   XPARSER_BASE_URL  - override download base URL
#   INSTALL_DIR       - install directory (default: ~/.local/bin)

VERSION="${XPARSER_VERSION:-latest}"
BASE_URL="${XPARSER_BASE_URL:-https://dllf.intsig.net/download/2026/Solution/xparse-cli}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# ── helpers ──

info()  { printf '  %s\n' "$*"; }
ok()    { printf '  ✓ %s\n' "$*"; }
err()   { printf '  ✗ %s\n' "$*" >&2; }

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)      err "Unsupported OS: $OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *)              err "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

ensure_dir() {
    if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
        return
    fi

    if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
        return
    fi

    if command -v sudo >/dev/null 2>&1; then
        info "Elevated permissions required for ${INSTALL_DIR}"
        sudo mkdir -p "$INSTALL_DIR"
        sudo chown "$(id -u):$(id -g)" "$INSTALL_DIR"
    else
        err "Cannot create ${INSTALL_DIR} and sudo not available"
        err "Try: INSTALL_DIR=~/.local/bin source <(curl -fsSL ${BASE_URL}/install.sh)"
        exit 1
    fi
}

ensure_path() {
    # Already in PATH
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) return ;;
    esac

    # Detect user's login shell
    SHELL_NAME="$(basename "${SHELL:-sh}")"
    case "$SHELL_NAME" in
        zsh)  PROFILE="$HOME/.zshrc" ;;
        bash) PROFILE="${HOME}/.bashrc" ;;
        fish) PROFILE="$HOME/.config/fish/config.fish" ;;
        *)    PROFILE="$HOME/.profile" ;;
    esac

    # Already configured in profile
    if grep -qF "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
        export PATH="${INSTALL_DIR}:$PATH"
        return
    fi

    # Build export line
    if [ "$SHELL_NAME" = "fish" ]; then
        EXPORT_LINE="set -gx PATH ${INSTALL_DIR} \$PATH"
    else
        EXPORT_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi

    mkdir -p "$(dirname "$PROFILE")" 2>/dev/null || true
    printf '\n# Added by xparse-cli installer\n%s\n' "$EXPORT_LINE" >> "$PROFILE"
    export PATH="${INSTALL_DIR}:$PATH"
    ok "Added ${INSTALL_DIR} to PATH in ${PROFILE}"
}

download_and_install() {
    BINARY="xparse-cli-${OS}-${ARCH}"
    URL="${BASE_URL}/${VERSION}/${BINARY}"
    DEST="${INSTALL_DIR}/xparse-cli"
    TMP="$(mktemp)"
    trap 'rm -f "$TMP"' EXIT

    # Check if upgrading
    if [ -x "$DEST" ]; then
        OLD_VER="$("$DEST" version 2>/dev/null | head -1 || echo "unknown")"
        info "Upgrading from ${OLD_VER}..."
    fi

    info "Downloading xparse-cli ${VERSION} for ${OS}/${ARCH}..."
    info "${URL}"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$TMP" "$URL"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TMP" "$URL"
    else
        err "curl or wget required"
        exit 1
    fi

    if [ ! -s "$TMP" ]; then
        err "Download failed or file is empty"
        exit 1
    fi

    ensure_dir

    mv "$TMP" "$DEST"
    chmod +x "$DEST"

    ensure_path

    echo ""
    echo "  =========================================================="
    printf "  "; "$DEST" version
    echo "  =========================================================="
    echo ""
    ok "Installed: ${DEST}"
}

# ── main ──

detect_platform
download_and_install
