#!/usr/bin/env bash

if [ -z $A01DROID_BUILD_SOURCE ]; then
    echo Missing environment variable A01DROID_BUILD_SOURCE 
    exit 1
fi

if ! [ -d $A01DROID_BUILD_SOURCE ]; then
    echo A01DROID_BUILD_SOURCE is not a directory
    exit 1
fi

mkdir -p build
cp -R $A01DROID_BUILD_SOURCE/* ./build/

version=`cat ./version`

docker build -t a01droid:$version-alpine-python3.6 -f dockerfiles/Dockerfile-alpine-python3.6 .
docker build -t a01droid:$version-jessie-python3.6 -f dockerfiles/Dockerfile-jessie-python3.6 .
