#!/bin/sh
cd `dirname $0`

tar -czf module.tar.gz setup.sh requirements.txt src meta.json
