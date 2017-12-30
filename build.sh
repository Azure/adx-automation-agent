#!/usr/bin/env bash

if [ -z $A01DROID_BUILD_SOURCE ]; then
    echo Missing environment variable A01DROID_BUILD_SOURCE 
    exit 1
fi

if ! [ -d $A01DROID_BUILD_SOURCE ]; then
    echo A01DROID_BUILD_SOURCE is not a directory
    exit 1
fi

mkdir -p material/build
cp -R $A01DROID_BUILD_SOURCE/* material/build/

version=`cat ./version`
docker build -t a01droid:$version .

