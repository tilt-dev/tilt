#!/usr/bin/env python

from collections import namedtuple
import datetime
import functools
import os
import random
import signal
import string
import subprocess
from typing import List


RESULTS_BLOCKLTR = '''  ____                 _ _
 |  _ \ ___  ___ _   _| | |_ ___ _
 | |_) / _ \/ __| | | | | __/ __(_)
 |  _ <  __/\__ \ |_| | | |_\__ \_
 |_| \_\___||___/\__,_|_|\__|___(_)'''


GOPATH = os.environ['GOPATH'] if 'GOPATH' in os.environ else os.path.join(os.environ['HOME'], 'go')
BLORGLY_BACKEND_DIR = os.path.join(GOPATH, 'src/github.com/windmilleng/blorgly-backend')
SERVICE_NAME = 'blorgly_backend_local'
TOUCHED_FILES = []
OUTPUT_WAIT_TIMEOUT_SECS = 10  # max time we'll wait on a process for output

# TODO(maia): capture amount of tilt overhead (i.e. total time - local build time)
Case = namedtuple('Case', ['name', 'setup', 'test'])
Result = namedtuple('Result', ['name', 'time_seconds'])

tilt_up_called = False
tilt_up_cmd = ["tilt", "up", SERVICE_NAME, '-d']
tilt_up_watch_cmd = ["tilt", "up", SERVICE_NAME, '--watch', '-d']


class Timer:
    def __enter__(self):
        self.start = datetime.datetime.now()
        return self

    def __exit__(self, *args):
        self.duration_secs = secs_since(self.start)


def main():
    print('NOTE: this script doesn\'t install `tilt` for you, and relies on you having the ' 
          'blorgly-backend project in your $GOPATH (`github.com/windmilleng/blorgly-backend`)')
    print()

    os.chdir(BLORGLY_BACKEND_DIR)

    cases = [
        make_case_tilt_up_once(),
        make_case_tilt_up_again_no_change(),
        make_case_tilt_up_again_new_file(),
        make_case_watch(),
    ]
    results = []

    try:
        for c in cases:
            print('~~ RUNNING CASE: {}'.format(c.name))
            args = []
            kwargs = {}

            print('~~~~ setup: {}'.format(c.name))
            ret = c.setup()
            if ret is not None:
                args = ret[0]
                kwargs = ret[1]

            print('~~~~ test: {}'.format(c.name))
            timetake = c.test(*args, **kwargs)

            results.append(Result(c.name, timetake))

        print()
        print(RESULTS_BLOCKLTR)
        print()

        for res in results:
            print('\t{} --> {:.5f} seconds'.format(res.name, res.time_seconds))
    finally:
        clean_up()


def run_and_wait_for_stdout(cmd: List[str], s: str, kill_on_match=False):
    # TODO(maia): do we also need to watch stderr?
    process = subprocess.Popen(cmd, stdout=subprocess.PIPE)
    wait_for_stdout(process, s, kill_on_match)
    return process


def wait_for_stdout(process: subprocess.Popen, s: str, kill_on_match=False):
    """
    Watch stdout of the given process for a line containing expected string `s`.
    If process isn't running at the start of this func, or if process exits without
    us finding `s` in its stdout, throw an error.

    If `kill_on_match`, kill the process once we find `s` in the output.
    """
    process.poll()  # make sure we have the latest return code info
    if process.returncode is not None:
        raise Exception('Process {} is no longer running (exit code {}), can\'t wait on stdout'.
                        format(process.args, process.returncode))

    while True:
        output = get_stdout_with_timeout(process)
        if output == '' and process.poll() is not None:
            break
        if output:
            print(output)
            if s in output:
                if kill_on_match:
                    process.terminate()
                return

    # if we got here, means process exited and we didn't find the string we were looking for
    rc = process.poll()
    raise Exception('Process {} exited with code {} and we didn\'t find expected '
                    'string "{}" in output'.format(process.args, rc, s))


def get_stdout_with_timeout(proc: subprocess.Popen):
    def _handle_timeout(signum, frame):
        raise TimeoutError('Timed out while waiting for output from process {}'.
                           format(proc.args))

    signal.signal(signal.SIGALRM, _handle_timeout)
    signal.alarm(OUTPUT_WAIT_TIMEOUT_SECS)
    try:
        return proc.stdout.readline().decode('utf-8').strip()
    finally:
        signal.alarm(0)


def make_case_tilt_up_once() -> Case:
    def set_tilt_up_called():
        global tilt_up_called
        tilt_up_called = True

    return Case("tilt up 1x", set_tilt_up_called,
                functools.partial(time_call, tilt_up_cmd))


def make_case_tilt_up_again_no_change() -> Case:
    def tilt_up_if_not_called():
        global tilt_up_called
        if tilt_up_called:
            print('Initial `tilt up` already called, no setup required')
            return
        print('Initial call to `tilt up`')
        call_or_error(tilt_up_cmd)

    return Case("tilt up again, no change", tilt_up_if_not_called,
                functools.partial(time_call, tilt_up_cmd))


def make_case_tilt_up_again_new_file() -> Case:
    def tilt_up_if_not_called():
        global tilt_up_called
        if not tilt_up_called:
            print('Initial call to `tilt up`')
            call_or_error(tilt_up_cmd)

        write_file(1000)  # 1KB

    return Case("tilt up again, new file", tilt_up_if_not_called,
                functools.partial(time_call, tilt_up_cmd))


def make_case_watch() -> Case:
    # TODO: make sure `tilt up --watch` isn't already running?
    def tilt_watch_and_wait_for_initial_build():
        tilt_proc = run_and_wait_for_stdout(tilt_up_watch_cmd, '[timing.py] finished initial build')

        # change a file
        write_file(1000)  # 1KB

        return [tilt_proc], {}

    def time_wait_for_next_build(proc: subprocess.Popen) -> float:
        with Timer() as t:
            wait_for_stdout(proc, '[timing.py] finished build from file change',
                            kill_on_match=True)
        return t.duration_secs

    return Case("watch file changed", tilt_watch_and_wait_for_initial_build,
                time_wait_for_next_build)


def time_call(cmd):
    """
        Call the given command (a list of strings representing command and args),
        return time in seconds.
    """
    with Timer() as t:
        call_or_error(cmd)

    return t.duration_secs


def call_or_error(cmd):
    """
        Call the given command (a list of strings representing command and args),
        raising an error if it fails.
    """
    return_code = subprocess.call(cmd)
    if return_code != 0:
        raise Exception('Command {} exited with exit code {}'.format(cmd, return_code))


def write_file(n):
    """
    Create a new file in the cwd containing the given number of
    byes (randomly generated).
    """
    name = '{}-{}'.format('timing_script', randstr(10))
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


def secs_since(t: datetime.datetime) -> float:
    return(datetime.datetime.now() - t).total_seconds()


if __name__ == "__main__":
    main()
