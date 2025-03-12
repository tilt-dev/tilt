#!/bin/bash
#
# Tilt installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash

# When releasing Tilt, the releaser should update this version number
# AFTER they upload new binaries.
VERSION="0.34.0"
BREW=$(command -v brew)

set -e

function copy_binary() {
  if [[ ":$PATH:" == *":$HOME/.local/bin:"* ]]; then
      if [ ! -d "$HOME/.local/bin" ]; then
        mkdir -p "$HOME/.local/bin"
      fi
      mv tilt "$HOME/.local/bin/tilt"
  else
      echo "Installing Tilt to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Tilt without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv tilt /usr/local/bin/tilt
  fi
}

function brew_install_or_upgrade() {
  set -x
  brew bundle --file=- <<< "brew 'tilt'"

  set +x
  location=$(command -v tilt)
  brew_root=$(brew --prefix)
  brew_tilt="$brew_root/bin/tilt"
  if [[ "$location" != "$brew_root"* ]]; then
      echo "Warning: you have a conflicting binary at: $location"
      echo "  Brew installed Tilt as: $brew_tilt"
      echo ""
      echo "  If you want to use the Tilt you just installed, you can:"
      echo "  1) Remove the other binary: rm $location"
      echo "  2) Adjust your PATH to put Brew first: export PATH=\"$brew_root/bin:\$PATH\""
      echo "  3) Alias tilt: alias tilt=$brew_tilt"
      exit 1
  fi
}

function install_tilt() {
  if [[ "$OSTYPE" == "linux"* ]]; then
      if [[ "$BREW" != "" ]]; then
          brew_install_or_upgrade

          # linux-homebrew is relatively recent. Make sure that tilt
          # under $HOME/.local/bin isn't overriding the homebrew one.
          rm -f "$HOME/.local/bin/tilt" || true
      else
          # On Linux, "uname -m" reports "aarch64" on ARM 64 bits machines,
          # and armv7l on ARM 32 bits machines like the Raspberry Pi.
          # This is a small workaround so that the install script works on ARM.
          case $(uname -m) in
              aarch64) ARCH=arm64;;
              armv7l)  ARCH=arm;;
              *)       ARCH=$(uname -m);;
          esac
          set -x
          curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$VERSION/tilt.$VERSION.linux.$ARCH.tar.gz | tar -xzv tilt
          copy_binary
      fi
  elif [[ "$OSTYPE" == "darwin"* ]]; then
      if [[ "$BREW" != "" ]]; then
          brew_install_or_upgrade
      else
          # On macOS, "uname -m" reports "arm64" on ARM 64 bits machines
          ARCH=$(uname -m)
          set -x
          curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$VERSION/tilt.$VERSION.mac.$ARCH.tar.gz | tar -xzv tilt
          copy_binary
      fi
  else
      set +x
      echo "The Tilt installer does not work for your platform: $OSTYPE"
      echo "For other installation options, check the following page:"
      echo "https://docs.tilt.dev/install.html#alternative-installations"
      echo "If you think your platform should be supported, please file an issue:"
      echo "https://github.com/tilt-dev/tilt/issues/new"
      echo "Thank you!"
      exit 1
  fi

  set +x
}

function version_check() {
  VERSION_FROM_BIN="$(tilt version 2>&1 || true)"
  RUBY_TILT_PATTERN="template engine not found"
  TILT_DEV_PATTERN='^v[0-9]+\.[0-9]+\.[0-9]+(-dev)?, built [0-9]+-[0-9]+-[0-9]+$'
  if [[ $VERSION_FROM_BIN =~ $RUBY_TILT_PATTERN ]]; then
    echo "Tilt installed!"
    echo
    echo "Note: the ruby templating program named 'tilt' (at $(command -v tilt)) appears before tilt.dev's tilt in your \$PATH."
    echo "You'll need to adjust your \$PATH, uninstall the other tilt, rename tilt, or use an absolute path to run tilt.dev's tilt. See https://docs.tilt.dev/faq.html."
    exit 1
  elif ! [[ $VERSION_FROM_BIN =~ $TILT_DEV_PATTERN ]]; then
    echo "Tilt installed!"
    echo
    echo "Note: it looks like it is not the first program named 'tilt' in your path. \`tilt version\` (running from $(command -v tilt)) did not return a tilt.dev version string."
    echo "It output this instead:"
    echo
    echo "$VERSION_FROM_BIN"
    echo
    echo "Perhaps you have a different program named tilt in your \$PATH?"
    exit 1
  else
    echo "Tilt installed!"
    echo "For the latest Tilt news, subscribe: https://tilt.dev/subscribe"
    echo "Run \`tilt up\` to start."
  fi
}

# so that we can skip installation in CI and just test the version check
if [[ -z $NO_INSTALL ]]; then
  install_tilt
fi

version_check

tilt verify-install

