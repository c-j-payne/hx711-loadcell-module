#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Install Python dependencies
python3 -m pip install --user -r "$SCRIPT_DIR/requirements.txt"
