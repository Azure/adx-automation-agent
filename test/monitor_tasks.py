import sys
import requests
import tabulate
import time
import datetime

try:
    _, store_host, run_id = sys.argv
except ValueError:
    print('Incorrect input. Expect inputs: job.py <store host> <run_id>')
    sys.exit(1)

print('Store host: {}'.format(store_host))
print('    Run ID: {}'.format(run_id))

while True:
    resp = requests.get('http://{}/run/{}/tasks'.format(store_host, run_id))
    resp.raise_for_status()
    rows = [(task['id'],
             task['name'].rsplit('.')[-1],
             task['status'],
             task['result'],
             (task.get('result_details') or dict()).get('agent'),
             (task.get('result_details') or dict()).get('duration'))
            for task in resp.json()]
    print(datetime.datetime.now())
    print(tabulate.tabulate(rows, headers=('id', 'name', 'status', 'result', 'agent', 'duration(ms)')))
    time.sleep(1)
