#!/usr/bin/env bash

. ./build.sh
az acr login -n a01reg

for flavor in alpine-python3.6 jessie-python3.6; do
    docker tag a01droid:$version-$flavor a01reg.azurecr.io/a01droid:$version-$flavor
    docker push a01reg.azurecr.io/a01droid:$version-$flavor
done

