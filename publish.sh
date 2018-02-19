#!/bin/bash

# publish the droid when a version is set
version=$1

if [ -z $version ]; then
    echo "Skip publishing because the version string is missing." >&2
    exit 0
fi

for os in linux darwin windows; do
    az storage blob upload -c droid -f ./bin/$os/a01droid -n $version/$os/a01droid --validate-content
    az storage blob upload -c droid -f ./bin/$os/a01droid -n latest/$os/a01droid --validate-content
done

az storage blob list -c droid --prefix $version
az storage blob list -c droid --prefix latest