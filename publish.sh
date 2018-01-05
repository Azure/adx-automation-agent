#!/usr/bin/env bash

set -e

registry=${1:-azureclidev}
server=`az acr show -n $registry --query loginServer -otsv`

az acr login -n $registry

version=`cat version`
for flavor in `ls dockerfiles`; do
    image=$server/azurecli-a01-droid:$flavor-$version
    docker build -t $image -f dockerfiles/$flavor/Dockerfile .
    docker push $image
done
