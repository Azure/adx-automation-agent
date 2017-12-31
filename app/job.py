import json
import sys
import time
import requests

try:
    _, store_host, run_id = sys.argv
except ValueError:
    print('Incorrect input. Expect inputs: job.py <store host> <run_id>')
    sys.exit(1)

print('Store host: {}'.format(store_host))
print('    Run ID: {}'.format(run_id))

while True:
    resp = requests.post('http://{}/run/{}/checkout'.format(store_host, run_id))
    resp.raise_for_status()

    if resp.status_code == 204:
        print('No more task')
        sys.exit(0)

    print('Running task >>>')
    print(json.dumps(resp.json(), indent=2))

    # mock test execution
    time.sleep(5)
