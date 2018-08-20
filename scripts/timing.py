#!/usr/bin/env python

from collections import namedtuple
import datetime
import functools
import os
from subprocess import call

RESULTS_BLOCKLTR = '''  ____                 _ _
 |  _ \ ___  ___ _   _| | |_ ___ _
 | |_) / _ \/ __| | | | | __/ __(_)
 |  _ <  __/\__ \ |_| | | |_\__ \_
 |_| \_\___||___/\__,_|_|\__|___(_)'''

BLORGLY_BACKEND_DIR = os.path.join(os.environ['GOPATH'], 'src/github.com/windmilleng/blorgly-backend')
SERVICE_NAME = 'blorgly_backend_local'

Result = namedtuple('Result', ['name', 'time_seconds'])


def main():
    print 'NOTE: this script doesn\'t install `tilt` for you, and relies on you having the ' \
          'blorgly-backend project in your $GOPATH (`github.com/windmilleng/blorgly-backend`)'
    print

    os.chdir(BLORGLY_BACKEND_DIR)

    results = []
    tilt_up_called = False

    # TEST CASE: `tilt up` 1x
    tilt_up = ["tilt", "up", SERVICE_NAME, '-d']
    time_tilt_up = functools.partial(time_call, tilt_up)

    def tilt_up_was_called():
        global tilt_up_called
        tilt_up_called = True
        
    timetake = run_test_case(tilt_up_called, time_tilt_up)
    results.append(Result("tilt up x 1", timetake))

    print
    print RESULTS_BLOCKLTR
    print

    for res in results:
        print '\t%s --> %f seconds' % (res.name, res.time_seconds)


def run_test_case(setup, test):
    if setup is not None:
        setup()

    return test()


def time_call(cmd):
    """
        Call the given command (a list of strings representing command and args),
        return time in seconds.
    """

    start = datetime.datetime.now()
    call(cmd)
    end = datetime.datetime.now()

    return (end - start).total_seconds()


if __name__ == "__main__":
    main()
