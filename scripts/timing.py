#!/usr/bin/env python

from enum import Enum
import datetime
import os
import random
import signal
import string
import subprocess
from typing import List, Callable


RESULTS_BLOCKLTR = '''  ____                 _ _
 |  _ \ ___  ___ _   _| | |_ ___ _
 | |_) / _ \/ __| | | | | __/ __(_)
 |  _ <  __/\__ \ |_| | | |_\__ \_
 |_| \_\___||___/\__,_|_|\__|___(_)'''

# SIZES
B = 1
KB = 1000 * B
MB = 1000 * KB

GOPATH = os.environ['GOPATH'] if 'GOPATH' in os.environ else os.path.join(os.environ['HOME'], 'go')
BLORGLY_BACKEND_DIR = os.path.join(GOPATH, 'src/github.com/windmilleng/blorgly-backend')
SERVICE_NAME = 'blorgly_backend_local'
TOUCHED_FILES = []
OUTPUT_WAIT_TIMEOUT_SECS = 10  # max time we'll wait on a process for output

tilt_up_called = False
tilt_up_cmd = ["tilt", "up", SERVICE_NAME, '-d']
tilt_up_watch_cmd = ["tilt", "up", SERVICE_NAME, '--watch', '-d']

# TODO(maia): capture amount of tilt overhead (i.e. total time - local build time)


class K8sEnv(Enum):
    GKE = 1
    D4M = 2
    MINIKUBE = 3


class Case:
    def __init__(self, name: str, func: Callable[[], float]):
        self.name = name
        self.func = func
        self.time_seconds = None

    def run(self):
        print('~~ RUNNING CASE: {}'.format(self.name))
        self.time_seconds = self.func()


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
        Case('tilt up 1x', test_tilt_up_once),
        Case('tilt up again, no change', test_tilt_up_again_no_change),
        Case('tilt up again, new file', test_tilt_up_again_new_file),
        Case('watch build from changed file', test_watch_build_from_changed_file),
        Case('tilt up, big file (5MB)', test_tilt_up_big_file),

        # Leave this commented out unless you particularly want it, it's damn slow.
        # Case('tilt up, REALLY big file (500MB)', test_tilt_up_really_big_file),
    ]

    try:
        for c in cases:
            c.run()

        print()
        print(RESULTS_BLOCKLTR)
        env = get_k8s_env()
        print('(Kubernetes environment: {})'.format(env.name))
        print()

        for c in cases:
            print('\t{} --> {:.5f} seconds'.format(c.name, c.time_seconds))
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


def time_call(cmd: List[str]):
    """
        Call the given command (a list of strings representing command and args),
        return time in seconds.
    """
    with Timer() as t:
        call_or_error(cmd)

    return t.duration_secs


def call_or_error(cmd: List[str]):
    """
        Call the given command (a list of strings representing command and args),
        raising an error if it fails.
    """
    return_code = subprocess.call(cmd)
    if return_code != 0:
        raise Exception('Command {} exited with exit code {}'.format(cmd, return_code))


def get_k8s_env() -> K8sEnv:
    """Get current Kubernetes env. (or throw an exception)."""
    out = subprocess.check_output(['kubectl', 'config', 'current-context'])

    outstr = out.decode('utf-8').strip()
    if outstr == 'docker-for-desktop':
        return K8sEnv.D4M
    elif 'gke' in outstr:
        return K8sEnv.GKE
    elif outstr == 'minikube':
        return K8sEnv.MINIKUBE
    else:
        raise Exception('Unable to find a matching k8s env for output "{}"'. format(outstr))


def write_file(n: int):
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


### THE TEST CASES
def test_tilt_up_once() -> float:
    # Set-up: note that tilt up has been called so we can skip setup for later tests
    global tilt_up_called
    tilt_up_called = True

    return time_call(tilt_up_cmd)


def test_tilt_up_again_no_change() -> float:
    tilt_up_if_not_called()

    return time_call(tilt_up_cmd)


def test_tilt_up_again_new_file() -> float:
    tilt_up_if_not_called()

    write_file(KB)

    return time_call(tilt_up_cmd)


def test_watch_build_from_changed_file() -> float:
    # TODO: make sure `tilt up --watch` isn't already running?

    # run `tilt up --watch` and wait for it to finish the initial build
    tilt_proc = run_and_wait_for_stdout(tilt_up_watch_cmd, '[timing.py] finished initial build')

    # change a file
    write_file(1000)  # 1KB

    with Timer() as t:
        wait_for_stdout(tilt_proc, '[timing.py] finished build from file change',
                        kill_on_match=True)
    return t.duration_secs


def test_tilt_up_big_file() -> float:
    write_file(5 * MB)

    return time_call(tilt_up_cmd)


def test_tilt_up_really_big_file() -> float:
    write_file(500 * MB)

    return time_call(tilt_up_cmd)


def tilt_up_if_not_called():
    global tilt_up_called
    if tilt_up_called:
        print('Initial `tilt up` already called, no setup required')
    else:
        print('Initial call to `tilt up`')
        call_or_error(tilt_up_cmd)
        tilt_up_called = True


### UTILS
def randstr(n: int) -> str:
    chars = string.ascii_uppercase + string.ascii_lowercase + string.digits
    return ''.join(random.choice(chars) for _ in range(n))


def randbytes(n: int) -> bytearray:
    return bytearray(os.urandom(n))


def secs_since(t: datetime.datetime) -> float:
    return(datetime.datetime.now() - t).total_seconds()


if __name__ == "__main__":
    main()
