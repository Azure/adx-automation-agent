# A01 Test Image Spec

To make a test image compatible to A01 live automation system, besides install executable test and product code, it should have following files and executables under the `/app` directory.

``` TEXT
-rwxr-xr-x 1 root root      147 Mar  2 21:32 get_index
-rw-r--r-- 1 root root      350 Mar  2 21:32 metadata.yml
-rwxr-xr-x 1 root root      618 Mar  2 21:32 prepare_pod
-rwxr-xr-x 1 root root      474 Mar  2 21:32 after_test
```

_The a01droid and a01dispatcher executables are no longer needed_

## Manifest /app/metadata.yml

The metadata.yml defines how to insert this image in a Kubernete Pod template.

Following is the Azure CLI project's the metadata.yml:

``` yaml
kind: DroidMetadata
version: v3
product: azurecli
storage: true
environments:
  - name: A01_SP_USERNAME
    type: secret
    value: sp.username
  - name: A01_SP_PASSWORD
    type: secret
    value: sp.password
  - name: A01_SP_TENANT
    type: secret
    value: sp.tenant
  - name: AZURE_TEST_RUN_LIVE
    type: argument-switch-live
    value: "True"
```

- The `kind` MUST be `DroidMetadata`
- The `version` MUST be `v3`
- The `product` MUST exist. It is the string represent your product. The value will be mapped to the name of the [kubernete secret](https://kubernetes.io/docs/concepts/configuration/secret/) in the cluster. It MUST be unique.
- The `storage` is a boolean. If is true, an Azure File Share will be mounted at `/mnt/storage` in the container.
- The `environment` is an array.
  - Each item contains `name`, `value`, and `type` properties.
  - The `name` specifues the environment variable name
  - The `type` specifies the source of the value.
    - The `secret` means the value comes from a Kubernetes secret. The `value` specify a key in the secret (secret is like an dictionary.)
    - The `argument-switch-live` means the environment variable is created if the run was create with `--live` option with CLI.
    - The `argument-value-mode` means the environment variable value is set by `--mode` option with CLI.

## Executable /app/get_index

The executable must returns test manifest in a JSON format. Its implementation is irrelevent. It can be a bash script, python script (with correct [shebang](https://en.wikipedia.org/wiki/Shebang_(Unix))), or any other program.

It MUST print a JSON array in following format to the `stderr`

``` JSON
[
  {
    "ver": "1.0",
    "execution": {
      "command": "python -m unittest azure.cli.core.tests.test_util.TestUtils.test_truncate_text_not_needed",
      "recording": null
    },
    "classifier": {
      "identifier": "azure.cli.core.tests.test_util.TestUtils.test_truncate_text_not_needed",
      "type": "Unit"
    }
  },
  {
    "ver": "1.0",
    "execution": {
      "command": "python -m unittest azure.cli.core.tests.test_util.TestUtils.test_truncate_text_zero_width",
      "recording": null
    },
    "classifier": {
      "identifier": "azure.cli.core.tests.test_util.TestUtils.test_truncate_text_zero_width",
      "type": "Unit"
    }
  }
]
```

- The top level value MUST be an array.
- Each item of the array MUST be a dictionary.
- Each item must contains three properties: `ver`, `exeuction`, and `classifier`.
- The `ver` MUST be set to `"1.0"`
- The `execution` MUST be a dictionary.
  - It MUST contains a `command` property.
  - The value of the `command` property is the command runs a specific test.
  - The `execution` can contains other properties.
- The `classifier` MUST be a dictionary.
  - It MUST contains a `identifier` property.
  - The value of the `identifier` property is a string used to identify a test. The test query is tested against it.
  - The `classifier` can contains other properties.

## Executable /app/prepare_pod

The `/app/prepared_pod` executable is run once after the pod is started. It can be used set up environment.

## Executable /app/after_test

The `/app/prepared_pod` executable is run once after each test. It can be used to clean up and save results. Two parameters are passed on to the script:

- The mount path to the file share
- The body of the test definition (see /app/get_index)

Here's an example:

``` bash
#!/bin/bash

mount_path=$1
task=$2

recording_path=$(echo $task | jq -r ".settings.execution.recording")
if [ "$recording_path" == "null" ]; then
    echo "Skip copying recording file. Code 1."
    exit 0
fi

if [ -z "$recording_path" ]; then
    echo "Skip copying recording file. Code 2."
    exit 0
fi

run_id=$(echo $task | jq -r ".run_id")
task_id=$(echo $task | jq -r ".id")

mkdir -p $mount_path/$run_id
cp $recording_path $mount_path/$run_id/recording_$task_id.yaml
```