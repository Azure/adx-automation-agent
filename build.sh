#!/bin/bash

root=$(cd `dirname $0`; pwd)
version=$1
echo "Version: ${version:=`date -u +local-%Y%m%d-%H%M%S`}"

for os in linux darwin windows; do
    export GOOS=$os
    mkdir -p $root/bin/$os
    echo -n "Building droid for $os ..."
    go build -o $root/bin/$os/a01droid -ldflags "-X main.version=$version"
    echo "Done."
done
