#!/bin/bash

# publish the droid when a version is set
version=$TRAVIS_TAG

if [ -z $version ]; then
    echo "Skip publishing because the version string is missing." >&2
    exit 0
fi

for os in linux ; do  # add darwin windows in the future
    az storage share create -n $os-$version --quota 1
    az storage share create -n $os-latest --quota 1

    az storage file upload -s $os-$version -f ./bin/$os/a01droid -n a01droid --validate-content --no-progress
    az storage file upload -s $os-$version -f ./bin/$os/a01dispatcher -n a01dispatcher --validate-content --no-progress
    az storage file upload -s $os-latest -f ./bin/$os/a01droid -n a01droid --validate-content --no-progress
    az storage file upload -s $os-latest -f ./bin/$os/a01dispatcher -n a01dispatcher --validate-content --no-progress
done
