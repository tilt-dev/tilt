#!/bin/bash
#
# Tilt installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash

# When releasing Tilt, the releaser should update this version number
# AFTER they upload new binaries.
VERSION="0.17.10"
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

function install_tilt() {
  if [[ "$OSTYPE" == "linux"* ]]; then
      if [[ "$BREW" != "" ]]; then
          set -x
          brew tap tilt-dev/tap
          brew install tilt-dev/tap/tilt

          # linux-homebrew is relatively recent. Make sure that tilt
          # under $HOME/.local/bin isn't overriding the homebrew one.
          rm -f "$HOME/.local/bin/tilt" || true
      else
          set -x
          curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$VERSION/tilt.$VERSION.linux.x86_64.tar.gz | tar -xzv tilt
          copy_binary
      fi
  elif [[ "$OSTYPE" == "darwin"* ]]; then
      if [[ "$BREW" != "" ]]; then
          set -x
          brew tap tilt-dev/tap
          brew install tilt-dev/tap/tilt
      else
          set -x
          curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$VERSION/tilt.$VERSION.mac.x86_64.tar.gz | tar -xzv tilt
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
    echo "Tilt installed! Run \`tilt up\` to start."
  fi
}

# so that we can skip installation in CI and just test the version check
if [[ -z $NO_INSTALL ]]; then
  install_tilt
fi

version_check

tilt verify-install

