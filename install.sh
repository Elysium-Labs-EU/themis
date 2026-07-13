#!/bin/bash
set -euo pipefail

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly DIM='\033[2m'
readonly NC='\033[0m' # No Color

# Configuration
readonly REPO="Elysium_Labs/themis"
readonly CODEBERG_URL="https://codeberg.org"
readonly BINARY_NAME="themis"
readonly INSTALL_DIR="${THEMIS_INSTALL_DIR:-/usr/local/bin}"

AUTO_YES=false

# Print functions
info() {
    echo -e "${BLUE}${BOLD}info${NC} $1"
}

success() {
    echo -e "${GREEN}${BOLD}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}${BOLD}warning${NC} $1"
}

error() {
    echo -e "${RED}${BOLD}error${NC} $1" >&2
}

step() {
    echo -e "\n${CYAN}${BOLD}→${NC} $1"
}

dim() {
    echo -e "${DIM}$1${NC}"
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --local <path>    Use a local binary instead of downloading from Codeberg"
    echo "  --help            Show this help message"
    echo "  --yes, -y         Skip all confirmation prompts (non-interactive mode)"
    echo ""
    echo "Environment variables:"
    echo "  THEMIS_INSTALL_DIR   Install directory (default: /usr/local/bin)"
    echo "  THEMIS_VERSION       Version to install (default: latest)"
}

confirm() {
    local prompt="$1"
    local default="${2:-n}"

    if [ "$AUTO_YES" = true ]; then
        [[ "$default" =~ ^[Yy]$ ]]
        return $?
    fi

    local response

    if [ "$default" = "y" ]; then
        prompt="$prompt [Y/n]"
    else
        prompt="$prompt [y/N]"
    fi

    echo -ne "${YELLOW}?${NC} $prompt "
    read -r response

    response=${response:-$default}
    [[ "$response" =~ ^[Yy]$ ]]
}

check_root() {
    if [ $EUID -ne 0 ]; then
        error "This script must be run as root"
        dim "  Try: sudo $0"
        exit 1
    fi
}

detect_download_tool() {
    if command -v curl &> /dev/null; then
        echo "curl"
    elif command -v wget &> /dev/null; then
        echo "wget"
    else
        error "Neither curl nor wget is installed"
        echo ""
        echo "Please install one of them:"
        dim "  Debian/Ubuntu: apt-get install curl"
        dim "  RHEL/CentOS:   yum install curl"
        dim "  Alpine:        apk add curl"
        exit 1
    fi
}

download_file() {
    local url="$1"
    local output="$2"
    local tool="$3"

    if [ "$tool" = "curl" ]; then
        curl -fsSL -o "$output" "$url" 2>&1 | sed 's/^/  /'
    else
        wget -q --show-progress -O "$output" "$url" 2>&1 | sed 's/^/  /'
    fi
}

fetch_json_field() {
    local url="$1"
    local field="$2"
    local tool="$3"

    local response
    if [ "$tool" = "curl" ]; then
        response=$(curl -fsSL "$url")
    else
        response=$(wget -qO- "$url")
    fi

    echo "$response" | grep -o "\"$field\":\"[^\"]*\"" | sed -E 's/"[^"]+":"([^"]+)"/\1/' | head -1
}

check_lynis() {
    if command -v lynis &> /dev/null; then
        dim "  Lynis: found"
        return
    fi

    warn "Lynis not found on PATH — themis shells out to it for the audit"

    if ! command -v apt-get &> /dev/null; then
        dim "  Install it manually: https://cisofy.com/lynis/"
        return
    fi

    if confirm "Install lynis now via apt-get?" "y"; then
        apt-get update -qq && apt-get install -y lynis
        success "Installed lynis"
    else
        dim "  Install later: apt-get install lynis"
    fi
}

detect_arch() {
    local arch
    arch=$(uname -m)

    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            dim "  Supported: x86_64, aarch64/arm64"
            exit 1
            ;;
    esac
}

main() {
    # Parse arguments
    local local_binary=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --local)
                if [[ $# -lt 2 ]]; then
                    error "--local requires a path argument"
                    usage
                    exit 1
                fi
                local_binary="$2"
                shift 2
                ;;
            --local=*)
                local_binary="${1#*=}"
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            --yes|-y)
                AUTO_YES=true
                shift
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Validate local binary if specified
    if [ -n "$local_binary" ]; then
        if [ ! -f "$local_binary" ]; then
            error "Local binary not found: $local_binary"
            exit 1
        fi
    fi

    echo ""
    echo -e "${BOLD}themis installer${NC}"
    echo ""

    info "Running pre-flight checks..."
    check_root

    local download_tool
    download_tool=$(detect_download_tool)
    dim "  Download tool: $download_tool"

    local arch
    arch=$(detect_arch)
    dim "  Architecture: $arch"

    check_lynis

    if [ "$INSTALL_DIR" != "/usr/local/bin" ]; then
        dim "  Install directory: $INSTALL_DIR (custom)"
    fi

    echo ""

    # Version resolution - skip when using a local binary
    local version=""
    if [ -z "$local_binary" ]; then
        version="${THEMIS_VERSION:-}"
        if [ -z "$version" ]; then
            step "Fetching latest version..."
            version=$(fetch_json_field "${CODEBERG_URL}/api/v1/repos/${REPO}/releases?limit=1" "tag_name" "$download_tool")

            if [ -z "$version" ]; then
                error "Failed to fetch latest version"
                dim "  Set THEMIS_VERSION environment variable to specify manually"
                exit 1
            fi

            info "Latest version: ${BOLD}$version${NC}"
        else
            info "Using version: ${BOLD}$version${NC}"
        fi
    else
        info "Using local binary: ${BOLD}$local_binary${NC}"
    fi

    echo ""

    echo -e "${BOLD}Installation plan:${NC}"
    if [ -n "$local_binary" ]; then
        echo "  1. Use local binary: ${local_binary}"
    else
        echo "  1. Download binary from Codeberg"
    fi
    echo "  2. Install to ${INSTALL_DIR}/${BINARY_NAME}"
    echo ""

    if ! confirm "Continue with installation?" "y"; then
        info "Installation cancelled"
        exit 0
    fi

    # Get the binary - either from local path or download
    local tmp_binary
    if [ -n "$local_binary" ]; then
        tmp_binary="$local_binary"
        success "Using local binary"
    else
        echo ""
        step "Downloading ${BINARY_NAME} ${version} for linux-${arch}..."

        local download_url="${CODEBERG_URL}/${REPO}/releases/download/${version}/themis-linux-${arch}"
        tmp_binary="/tmp/${BINARY_NAME}"

        if ! download_file "$download_url" "$tmp_binary" "$download_tool"; then
            error "Download failed"
            dim "  URL: $download_url"
            exit 1
        fi

        if [ ! -f "$tmp_binary" ]; then
            error "Binary not found after download"
            exit 1
        fi

        success "Downloaded successfully"

        step "Verifying checksum..."
        local checksums_url="${CODEBERG_URL}/${REPO}/releases/download/${version}/sha256sums.txt"
        local tmp_checksums="/tmp/${BINARY_NAME}_sha256sums.txt"

        if ! download_file "$checksums_url" "$tmp_checksums" "$download_tool"; then
            error "Failed to download sha256sums.txt"
            exit 1
        fi

        local binary_name="themis-linux-${arch}"
        local expected_checksum
        expected_checksum=$(grep "  ${binary_name}$" "$tmp_checksums" | awk '{print $1}')

        if [ -z "$expected_checksum" ]; then
            error "No checksum found for ${binary_name} in sha256sums.txt"
            exit 1
        fi

        local actual_checksum
        actual_checksum=$(sha256sum "$tmp_binary" | awk '{print $1}')

        if [ "$expected_checksum" != "$actual_checksum" ]; then
            error "Checksum mismatch — binary may be corrupted"
            dim "  expected: $expected_checksum"
            dim "  got:      $actual_checksum"
            rm -f "$tmp_binary" "$tmp_checksums"
            exit 1
        fi

        rm -f "$tmp_checksums"
        success "Checksum verified"
    fi

    # Install binary
    step "Installing binary..."
    mkdir -p "$INSTALL_DIR"
    chmod +x "$tmp_binary"
    cp "$tmp_binary" "${INSTALL_DIR}/${BINARY_NAME}"
    success "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

    echo ""
    echo -e "${GREEN}${BOLD}Installation complete!${NC}"
    echo ""
    echo -e "${BOLD}Next steps:${NC}"
    echo "  1. Run a hardening audit:"
    echo -e "     ${CYAN}sudo themis check${NC}"
    echo ""
    echo "  2. Review what themis would apply:"
    echo -e "     ${CYAN}sudo themis plan${NC}"
    echo ""
    echo "  3. Apply fixes (rollback metadata saved automatically):"
    echo -e "     ${CYAN}sudo themis apply${NC}"
    echo ""
}

main "$@"
