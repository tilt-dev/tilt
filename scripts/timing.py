#!/usr/bin/env python

from collections import namedtuple
import datetime
import functools
import os
import random
import string
from subprocess import call


RESULTS_BLOCKLTR = '''  ____                 _ _
 |  _ \ ___  ___ _   _| | |_ ___ _
 | |_) / _ \/ __| | | | | __/ __(_)
 |  _ <  __/\__ \ |_| | | |_\__ \_
 |_| \_\___||___/\__,_|_|\__|___(_)'''


GOPATH = os.environ['GOPATH'] if 'GOPATH' in os.environ else os.path.join(os.environ['HOME'], 'go')
BLORGLY_BACKEND_DIR = os.path.join(GOPATH, 'src/github.com/windmilleng/blorgly-backend')
SERVICE_NAME = 'blorgly_backend_local'
TOUCHED_FILES = []

# TODO(maia): capture amount of tilt overhead (i.e. total time - local build time)
Case = namedtuple('Case', ['name', 'setup', 'test'])
Result = namedtuple('Result', ['name', 'time_seconds'])

tilt_up_called = False
tilt_up_cmd = ["tilt", "up", SERVICE_NAME, '-d']


def main():
    print('NOTE: this script doesn\'t install `tilt` for you, and relies on you having the ' \
          'blorgly-backend project in your $GOPATH (`github.com/windmilleng/blorgly-backend`)')
    print()

    os.chdir(BLORGLY_BACKEND_DIR)

    cases = [
        make_case_tilt_up_once(),
        make_case_tilt_up_again_no_change(),
        make_case_tilt_up_again_new_file(),
    ]
    results = []

    try:
        for c in cases:
            print('~~ RUNNING CASE: %s' % c.name)
            c.setup()
            timetake = c.test()
            results.append(Result(c.name, timetake))

        print()
        print(RESULTS_BLOCKLTR)
        print()

        for res in results:
            print('\t%s --> %f seconds' % (res.name, res.time_seconds))
    finally:
        clean_up()


def make_case_tilt_up_once():
    def set_tilt_up_called():
        global tilt_up_called
        tilt_up_called = True

    return Case("tilt up 1x", set_tilt_up_called,
                functools.partial(time_call, tilt_up_cmd))


def make_case_tilt_up_again_no_change():
    def tilt_up_if_not_called():
        global tilt_up_called
        if tilt_up_called:
            print('Initial `tilt up` already called, no setup required')
            return
        print('Initial call to `tilt up`')
        call_or_error(tilt_up_cmd)

    return Case("tilt up again, no change", tilt_up_if_not_called,
                functools.partial(time_call, tilt_up_cmd))


def make_case_tilt_up_again_new_file():
    def tilt_up_if_not_called():
        global tilt_up_called
        if not tilt_up_called:
            print('Initial call to `tilt up`')
            call_or_error(tilt_up_cmd)

        # TODO: clean this file up
        write_file(1000)  # 1KB

    return Case("tilt up again, new file", tilt_up_if_not_called,
                functools.partial(time_call, tilt_up_cmd))


def time_call(cmd):
    """
        Call the given command (a list of strings representing command and args),
        return time in seconds.
    """

    start = datetime.datetime.now()
    call_or_error(cmd)
    end = datetime.datetime.now()

    return (end - start).total_seconds()

def call_or_error(cmd):
    """
        Call the given command (a list of strings representing command and args),
        raising an error if it fails.
    """
    return_code = call(cmd)
    if return_code != 0:
        raise Exception('Command {} exited with exit code {}'.format(cmd, return_code))


def write_file(n):
    """
    Create a new file in the cwd containing the given number of
    byes (randomly generated).
    """
    name = '%s-%s' % ('timing_script', randstr(10))
    with open(name, 'w+b') as f:
        f.write(randbytes(n))

    # TODO(maia): this should be stored on an object instead of in a global var :-/
    global TOUCHED_FILES
    TOUCHED_FILES.append(name)


def clean_up():
    # delete any files we touched
    # TODO(maia): this info should be stored better than in a global var :-/
    global TOUCHED_FILES
    for f in TOUCHED_FILES:
        if os.path.isfile(f):
            os.remove(f)


# Some utils
def randstr(n):
    chars = string.ascii_uppercase + string.ascii_lowercase + string.digits
    return ''.join(random.choice(chars) for _ in range(n))


def randbytes(n):
    return bytearray(os.urandom(n))


if __name__ == "__main__":
    main()
