#!/usr/bin/env bash

. ./build.sh

docker tag a01droid:$version a01reg.azurecr.io/a01droid:$version
az acr login -n a01reg
docker push a01reg.azurecr.io/a01droid:$version

