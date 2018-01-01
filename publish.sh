#!/usr/bin/env bash

version=`cat version`
for flavor in `ls dockerfiles`; do
    docker build -t troydai/azurecli-a01-droid:$flavor-$version \
                 -f dockerfiles/$flavor/Dockerfile \
                 .
    docker push troydai/azurecli-a01-droid:$flavor-$version
done
