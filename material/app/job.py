import os
import json
import requests

store_uri = 'http://{}'.format(os.environ['A01DROID_STORE_HOST'])

runs = requests.get('{}/runs'.format(store_uri)).json()

print(json.dumps(runs, indent=2))

