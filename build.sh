#!/usr/bin/env bash

# build a local image for quick testing

set -e

version=`date '+%H%M%S'`
image=azurecli-a01-droid:local-$version
docker build -t $image -f dockerfiles/python3.6/Dockerfile .
