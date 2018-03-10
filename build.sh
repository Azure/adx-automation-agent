#!/bin/bash

root=$(cd `dirname $0`; pwd)
version=$TRAVIS_TAG
commit=${TRAVIS_COMMIT:-`git rev-parse --verify HEAD`}
echo "Version: ${version:=`date -u +local-%Y%m%d-%H%M%S`}"

export CGO_ENABLED=0

for os in linux; do  # add darwin and windows in the future
    export GOOS=$os
    mkdir -p $root/bin/$os
    echo -n "Building droid for $os ..."
    go build -o $root/bin/$os/a01droid \
             -ldflags "-X main.version=$version -X main.sourceCommit=$commit" \
             github.com/Azure/adx-automation-agent/droid
    go build -o $root/bin/$os/a01dispatcher \
             -ldflags "-X main.version=$version -X main.sourceCommit=$commit" \
             github.com/Azure/adx-automation-agent/dispatcher 
    chmod +x $root/bin/$os/a01droid
    chmod +x $root/bin/$os/a01dispatcher
    echo "Done."
done
