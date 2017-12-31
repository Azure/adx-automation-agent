import os
import sys
from datetime import datetime
from subprocess import check_output, CalledProcessError, STDOUT

import requests

try:
    _, store_host, run_id = sys.argv
except ValueError:
    print('Incorrect input. Expect inputs: job.py <store host> <run_id>')
    sys.exit(1)


def o(message):
    sys.stdout.write(message + '\n')
    sys.stdout.flush()


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

    begin = datetime.now()
    try:
        output = check_output(['python', '-m', 'unittest', task['settings']['path']], stderr=STDOUT)
        output = output.decode('utf-8')
        result = 'Passed'
    except CalledProcessError as error:
        output = error.output.decode('utf-8')
        result = 'Failed'

    elapsed = datetime.now() - begin

    o('Task output:')
    o(output)

    result_details = task.get('result_detail', dict())
    result_details['agent'] = '{}@{}'.format(
        os.environ.get('ENV_POD_NAME', 'N/A'),
        os.environ.get('ENV_NODE_NAME', 'N/A'))
    result_details['duration'] = int(elapsed.microseconds / 1000)

    patch = {
        'result': result,
        'result_details': result_details,
        'status': 'completed',
    }
    requests.patch('http://{}/task/{}'.format(store_host, task['id']), json=patch).raise_for_status()
