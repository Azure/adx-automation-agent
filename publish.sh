#!/bin/bash

# publish the droid when a version is set
version=$TRAVIS_TAG

if [ -z $version ]; then
    echo "Skip publishing because the version string is missing." >&2
    exit 0
fi

for os in linux ; do  # add darwin windows in the future
    sharename=${version//./-}
    az storage share create -n $os-$sharename --quota 1
    az storage share create -n $os-latest --quota 1

    az storage file upload -s $os-$sharename --source ./bin/$os/a01droid -p /a01droid --validate-content --no-progress
    az storage file upload -s $os-$sharename --source ./bin/$os/a01dispatcher -p /a01dispatcher --validate-content --no-progress
    az storage file upload -s $os-latest --source ./bin/$os/a01droid -p /a01droid --validate-content --no-progress
    az storage file upload -s $os-latest --source ./bin/$os/a01dispatcher -p /a01dispatcher --validate-content --no-progress
done
