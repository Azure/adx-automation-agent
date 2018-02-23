import os.path
from pkgutil import iter_modules
from unittest import TestLoader
from importlib import import_module
from json import dumps as json_dumps

import azure.cli
from azure.cli.testsdk import ScenarioTest, LiveScenarioTest

records = []

def get_test_type(test_case):
    if isinstance(test_case, ScenarioTest):
        return 'Recording'
    elif isinstance(test_case, LiveScenarioTest):
        return 'Live'
    return 'Unit'

def search(path, prefix=''):
    loader = TestLoader()
    for _, name, isPkg in iter_modules(path):
        full_name = '{}.{}'.format(prefix, name)
        module_path = os.path.join(path[0], name)

        if isPkg:
            search([module_path], full_name)

        if not isPkg and name.startswith('test'):
            m = import_module(full_name)
            for suite in loader.loadTestsFromModule(m):
                for test in suite._tests:
                    records.append({
                        'module': full_name,
                        'class': test.__class__.__name__,
                        'method': test._testMethodName,
                        'type': get_test_type(test),
                        'path': '{}.{}.{}'.format(full_name, test.__class__.__name__, test._testMethodName)})

search(azure.cli.__path__, 'azure.cli')

print(json_dumps(records, indent=2))