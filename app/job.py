import os
import sys
from datetime import datetime
from subprocess import check_output, CalledProcessError, STDOUT

import requests


def o(message):
    sys.stdout.write(message + '\n')
    sys.stdout.flush()


try:
    run_id = os.environ['A01_DROID_RUN_ID']
except KeyError:
    o('The environment variable A01_DROID_RUN_ID is missing.')
    sys.exit(1)

try:
    store_host = os.environ['A01_STORE_NAME']
except KeyError:
    store_host = 'a01store'
    o('The environment variable A01_STORE_NAME is missing. Fallback to a01store.')

run_live = os.environ.get('A01_RUN_LIVE', 'False') == 'True'
username = os.environ.get('A01_SP_USERNAME', None)
password = os.environ.get('A01_SP_PASSWORD', None)
tenant = os.environ.get('A01_SP_TENANT', None)

if run_live:
    if username and password and tenant:
        try:
            login = check_output(
                'az login --service-principal -u {} -p {} -t {}'.format(username, password, tenant).split())
            o(login.decode('utf-8'))
        except CalledProcessError as error:
            o('Failed to sign in with the service principal.')
            o(str(error))
            sys.exit(1)
    else:
        o('Missing service principal settings for live test')
        sys.exit(1)

o('Store host: {}'.format(store_host))
o('    Run ID: {}'.format(run_id))

while True:
    resp = requests.post('http://{}/run/{}/checkout'.format(store_host, run_id))
    resp.raise_for_status()

    if resp.status_code == 204:
        print('No more task')
        sys.exit(0)

    task = resp.json()
    o('Pick up task {}.'.format(task['id']))

    # update the running agent first
    result_details = task.get('result_detail', dict())
    result_details['agent'] = '{}@{}'.format(
        os.environ.get('ENV_POD_NAME', 'N/A'),
        os.environ.get('ENV_NODE_NAME', 'N/A'))
    result_details['live'] = run_live
    patch = {
        'result_details': result_details
    }
    requests.patch('http://{}/task/{}'.format(store_host, task['id']), json=patch).raise_for_status()

    # run the task
    begin = datetime.now()
    try:
        output = check_output(
            ['python', '-m', 'unittest', task['settings']['path']],
            stderr=STDOUT,
            env={'AZURE_TEST_RUN_LIVE': 'True'} if run_live else None)
        output = output.decode('utf-8')
        result = 'Passed'
    except CalledProcessError as error:
        output = error.output.decode('utf-8')
        result = 'Failed'
    elapsed = datetime.now() - begin

    o('Task output:')
    o(output)

    result_details['duration'] = int(elapsed.microseconds / 1000)
    patch = {
        'result': result,
        'result_details': result_details,
        'status': 'completed',
    }
    requests.patch('http://{}/task/{}'.format(store_host, task['id']), json=patch).raise_for_status()
