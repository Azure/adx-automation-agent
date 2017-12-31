import sys
import requests

try:
    _, store_host, run_id = sys.argv
except ValueError:
    print('Incorrect input. Expect inputs: job.py <store host> <run_id>')
    sys.exit(1)

print('Store host: {}'.format(store_host))
print('    Run ID: {}'.format(run_id))

resp = requests.get('http://{}/run/{}/tasks'.format(store_host, run_id))
resp.raise_for_status()

for task in resp.json():
    task['status'] = 'initialized'
    requests.patch('http://{}/task/{}'.format(store_host, task['id']), json={'status': 'initialized'})\
            .raise_for_status()
