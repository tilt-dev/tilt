#!/bin/bash
#
# Tilt installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/windmilleng/tilt/master/scripts/install.sh | bash

# When releasing Tilt, the releaser should update this version number
# AFTER they upload new binaries.
VERSION="0.11.3"
BREW=$(which brew)

set -e

if [[ "$OSTYPE" == "linux-gnu" ]]; then
    set -x
    curl -fsSL https://github.com/windmilleng/tilt/releases/download/v$VERSION/tilt.$VERSION.linux.x86_64.tar.gz | tar -xzv tilt
    sudo mv tilt /usr/local/bin/tilt

elif [[ "$OSTYPE" == "darwin"* ]]; then

    if [[ "$BREW" != "" ]]; then
        set -x
        brew tap windmilleng/tap
        brew install windmilleng/tap/tilt
    else
        set -x
        curl -fsSL https://github.com/windmilleng/tilt/releases/download/v$VERSION/tilt.$VERSION.mac.x86_64.tar.gz | tar -xzv tilt
        sudo mv tilt /usr/local/bin/tilt
    fi
else
    set +x
    echo "The Tilt installer does not work for your platform: $OS"
    echo "Please file an issue at https://github.com/windmilleng/tilt/issues/new"
    exit 1
fi

# TODO(nick): Add verification that Tilt installed successfully.

set +x
echo "Tilt installed! Run \`tilt up\` to start."
