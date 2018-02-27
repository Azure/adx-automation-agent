#!/bin/bash

root=$(cd `dirname $0`; pwd)
version=$1
commit=${TRAVIS_COMMIT:-`git rev-parse --verify HEAD`}
echo "Version: ${version:=`date -u +local-%Y%m%d-%H%M%S`}"

for os in linux darwin windows; do
    export GOOS=$os
    mkdir -p $root/bin/$os
    echo -n "Building droid for $os ..."
    go build -o $root/bin/$os/a01droid -ldflags "-X main.version=$version -X main.sourceCommit=$commit"
    chmod +x $root/bin/$os/a01droid
    echo "Done."
done
