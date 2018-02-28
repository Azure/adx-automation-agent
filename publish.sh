#!/bin/bash

# publish the droid when a version is set
version=$1

if [ -z $version ]; then
    echo "Skip publishing because the version string is missing." >&2
    exit 0
fi

for os in linux ; do  # add darwin windows in the future
    az storage blob upload -c droid -f ./bin/$os/a01droid -n $version/$os/a01droid --validate-content --no-progress
    az storage blob upload -c droid -f ./bin/$os/a01droid -n latest/$os/a01droid --validate-content --no-progress
    az storage blob upload -c droid -f ./bin/$os/a01dispatcher -n $version/$os/a01dispatcher --validate-content --no-progress
    az storage blob upload -c droid -f ./bin/$os/a01dispatcher -n latest/$os/a01dispatcher --validate-content --no-progress
done

az storage blob list -c droid --prefix $version -otable
az storage blob list -c droid --prefix latest -otable
