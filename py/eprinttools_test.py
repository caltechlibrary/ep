#!/usr/bin/env python3
import os
import sys
import eprinttools
import random

#
# Tests
#
def test_get_keys(t, eprint_url, auth_type = "", username = "", secret = ""):
    #t.verbose_on() # turn verboseness on for debugging
    cfg = eprinttools.cfg(eprint_url, auth_type, username, secret)
    keys = eprinttools.get_keys(cfg)
    if len(keys) == 0:
        t.error(f"Expected more than zero keys for {cfg}")
    else:
        t.print("found", len(keys), f"for {cfg}")


def test_get_metadata(t, eprint_url, auth_type = 0, username = "", secret = ""):
    test_name = t.test_name()
    #t.verbose_on() # turn verboseness on for debugging
    cfg = eprinttools.cfg(eprint_url, auth_type, username, secret)
    keys = eprinttools.get_keys(cfg)
    if len(keys) == 0:
        t.error(f"Can't test {test_name} without keys, got zero keys")
        return
    #FIXME: Pick some random keys to test getting metadata records!
    check_keys = []
    for i in range(100):
        key = random.choice(keys)
        if key not in check_keys:
            check_keys.append(key)
        if len(check_keys) > 50:
            break
    for key in check_keys:
        # We are going to try to get the metadata for the EPrint record but not store it in a dataset collectin...
        data = eprinttools.get_metadata(cfg, key, False)
        if len(data) == 0:
            e_msg = eprinttools.error_message()
            if e_msg.startswith("401") == False:
                t.error(f"Expected data for {key}, got {data}")
            else:
                t.print(f"found {key}, requires authentication")
        else:
            t.print(f"found {key} with data")


#
# Test harness
#
class ATest:
    def __init__(self, test_name, verbose = False):
        self._test_name = test_name
        self._error_count = 0
        self._verbose = verbose

    def test_name(self):
        return self._test_name

    def is_verbose(self):
        return self._verbose

    def verbose_on(self):
        self._verbose = True
       
    def verbose_off(self):
        self._verbose = False

    def print(self, *msg):
        if self._verbose == True:
            print(*msg)

    def error(self, *msg):
        fn_name = self._test_name
        self._error_count += 1
        print(f"{fn_name} failed, ", *msg)

    def error_count(self):
        return self._error_count

class TestRunner:
    def __init__(self, set_name):
        self._set_name = set_name
        self._tests = []
        self._error_count = 0

    def add(self, fn, params = []):
        self._tests.append((fn, params))

    def run(self):
        for test in self._tests:
            fn_name = test[0].__name__
            t = ATest(fn_name)
            fn, params = test[0], test[1]
            fn(t, *params)
            error_count = t.error_count()
            if error_count > 0:
                print(f"\t\t{fn_name} failed, {error_count} errors found")
            self._error_count += error_count
        error_count = self._error_count
        set_name = self._set_name
        if error_count > 0:
            print(f"Failed {set_name}, {error_count} total errors found")
            sys.exit(1)
        print("PASS")
        print("Ok", __file__)
        sys.exit(0)


def setup():
    eprint_url = os.getenv("EPRINT_URL")
    auth_type = os.getenv("EPRINT_AUTH_TYPE")
    username = os.getenv("EPRINT_USER")
    secret = os.getenv("EPRINT_PASSWD")
    if auth_type == None:
        auth_type = ""
    if username == None:
        username = ""
    if secret == None:
        secret = ""
    return eprint_url, auth_type, username, secret

#
# Run tests
#
if __name__ == "__main__":
    version = eprinttools.version()
    eprint_url, auth_type, username, secret = setup()
    if eprint_url == None:
        print(f"Skipping tests for eprinttools {version}, EPRINT_URL not set in the environment")
        sys.exit(0)
    test_runner = TestRunner(os.path.basename(__file__))
    test_runner.add(test_get_keys, [eprint_url, auth_type, username, secret])
    test_runner.add(test_get_metadata, [eprint_url, auth_type, username, secret])
    test_runner.run()

