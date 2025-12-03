#!/usr/bin/env bash
# shellcheck disable=SC2086 
# Quick install script for ingress-perf
# Downloads the latest release version based on system architecture and OS

set -euo pipefail

# Configuration
REPO="cloud-bulldozer/ingress-perf"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin/}"

# Detect OS
detect_os() {
  local os
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  
  case "${os}" in
    linux*)
      echo "linux"
      ;;
    darwin*)
      echo "darwin"
      ;;
    mingw* | msys* | cygwin*)
      echo "windows"
      ;;
    *)
      echo "Unsupported operating system: ${os}"
      exit 1
      ;;
  esac
}

# Get latest release version from GitHub
get_latest_version() {
  local version
  if command -v curl &> /dev/null; then
    version=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
              grep '"tag_name":' | \
              sed -E 's/.*"([^"]+)".*/\1/')
  else
    echo "curl command not found. Please install it."
    exit 1
  fi
  
  if [[ -z "${version}" ]]; then
    echo "Failed to fetch latest version"
    exit 1
  fi
  
  echo "${version}"
}

# Download and extract binary
download_and_extract() {
  local version=$1
  local os=$2
  local arch=$3
  local archive_name="ingress-perf-${os}-${version}-${arch}.tar.gz"
  local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
  mkdir -p ${INSTALL_DIR}
  echo "Downloading ingress-perf ${version} for ${os}/${arch}..."
  echo "URL: ${download_url}"
  curl -sL -f "${download_url}" | tar xz -C ${INSTALL_DIR} ingress-perf
}

# Verify installation
verify_installation() {
  if command -v ingress-perf &> /dev/null; then
    echo "ingress-perf is now available in your PATH, installed at ${INSTALL_DIR}"
  else
    echo "ingress-perf installed to ${INSTALL_DIR}, but not found in PATH"
    echo "You may need to add ${INSTALL_DIR} to your PATH"
  fi
}

# Main installation flow
main() {
  echo "Starting ingress-perf 🔥 installation..."
  
  # Detect system   
  local os
  local arch
  os=$(detect_os)
  arch=$(uname -m | sed s/aarch64/arm64/)
  
  echo "Detected system: ${os}/${arch}"
  
  # Get latest version
  local version
  version=$(get_latest_version)
  echo "Latest version: ${version}"
  
  # Download and extract
  download_and_extract "${version}" "${os}" "${arch}"
  
  # Verify
  verify_installation
  
  echo "Get started with: ingress-perf help"
}

# Run main function
main "$@"

