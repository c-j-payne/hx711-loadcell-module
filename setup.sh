#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Install Python dependencies with --break-system-packages for externally managed environments
python3 -m pip install --break-system-packages -r "$SCRIPT_DIR/requirements.txt"
