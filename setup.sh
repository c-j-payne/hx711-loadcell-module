#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Ensure pip is available
if ! python3 -m pip --version > /dev/null 2>&1; then
    echo "pip not found, installing..."
    apt-get update && apt-get install -y python3-pip
fi

# Install Python dependencies with --break-system-packages for externally managed environments
python3 -m pip install --break-system-packages -r "$SCRIPT_DIR/requirements.txt"
