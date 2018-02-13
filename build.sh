#!/bin/bash

root=$(cd `dirname $0`; pwd)

for os in linux darwin windows; do
    export GOOS=$os
    mkdir -p $root/bin/$os
    go build -o $root/bin/$os/a01droid
done
