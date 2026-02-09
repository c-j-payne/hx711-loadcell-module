#!/bin/sh
cd `dirname $0`

# Create a virtual environment to run our code
VENV_NAME="venv"
PYTHON="$VENV_NAME/bin/python"

if ! $PYTHON -m pip install pyinstaller -Uqq; then
    exit 1
fi

$PYTHON -m PyInstaller --onefile --hidden-import="googleapiclient" --hidden-import="lgpio" --collect-binaries=lgpio --collect-submodules=lgpio --add-data="src/hx711.py:." src/main.py
tar -czvf dist/archive.tar.gz meta.json ./dist/main
