#!/usr/bin/env python

from collections import namedtuple
from enum import Enum
import datetime
import os
import random
import signal
import string
import subprocess
import time
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
BLORG_FRONTEND_DIR = os.path.join(GOPATH, 'src/github.com/windmilleng/blorg-frontend')
BLORG_FRONTEND_INDEX = os.path.join(BLORG_FRONTEND_DIR, 'index.html')
BLORGLY_BACKEND_DIR = os.path.join(GOPATH, 'src/github.com/windmilleng/blorgly-backend')
SERVICE_NAME = 'blorgly_backend_local'
FE_SERVICE_NAME = 'blorg_frontend'
TOUCHED_FILES = []
OUTPUT_WAIT_TIMEOUT_SECS = 45  # max time we'll wait on a process for output


class Service:
    def __init__(self, name, work_dir, write_dir, main_go_path):
        self.name = name  # name of service as passed to `tilt up <name>`
        self.work_dir = work_dir  # set this as working dir when using this service (where Tiltfile lives)
        self.write_dir = write_dir  # write temp files to here
        self.main_go_path = main_go_path  # path to main.go

        with open(main_go_path, 'r') as f:
            # hold onto original contents of main.go so we can edit it/reset it
            self.main_go_orig_contents = f.read()

        self.touched_files = []
        self.main_go_changed = False
        self.up_called = False

    def tilt_up_cmd(self):
        return ["tilt", "up", self.name, '-d', '--browser=off']

    def tilt_up_watch_cmd(self):
        cmd = self.tilt_up_cmd()
        cmd.append('--watch')
        return cmd

    def write_file(self, n: int):
        """
        Create a new file in the designated write directory containing the given
        number of byes (randomly generated).
        """
        name = '{}-{}'.format('timing_script', randstr(10))
        with open(os.path.join(self.write_dir, name), 'w+b') as f:
            f.write(randbytes(n))

        self.touched_files.append(name)

    def change_main_go(self):
        with open(self.main_go_path, 'a') as gofile:
            gofile.write('\n// timing.py edit\n')
        self.main_go_changed = True


servantes_path = os.path.join(GOPATH, 'src/github.com/windmilleng/servantes')
SERVANTES_FE = Service("fe", servantes_path, os.path.join(servantes_path, 'servantes'),
                       os.path.join(servantes_path, 'servantes/main.go'))

SERVICES = [SERVANTES_FE]

tilt_up_called = False
tilt_up_cmd = ["tilt", "up", SERVICE_NAME, '-d', '--browser=off']
tilt_up_watch_cmd = ["tilt", "up", SERVICE_NAME, '--watch', '-d', '--browser=off']
tilt_up_fe_cmd = ["tilt", "up", FE_SERVICE_NAME, '-d', '--browser=off']
tilt_up_watch_fe_cmd = ["tilt", "up", FE_SERVICE_NAME, '--watch', '-d', '--browser=off']

# TODO(maia): capture amount of tilt overhead (i.e. total time - local build time)


class K8sEnv(Enum):
    GKE = 1
    D4M = 2
    MINIKUBE = 3


class Case:
    def __init__(self, name: str, serv: Service, func: Callable[[Service], float], wd=BLORGLY_BACKEND_DIR):
        self.name = name
        self.serv = serv
        self.func = func
        self.time_seconds = None
        self.wd = wd

    def run(self):
        os.chdir(self.serv.work_dir)
        print()
        print('~~ RUNNING CASE: {}'.format(self.name))
        self.time_seconds = self.func(self.serv)


class Timer:
    def __enter__(self):
        self.start = datetime.datetime.now()
        return self

    def __exit__(self, *args):
        self.duration_secs = secs_since(self.start)


def main():
    cases = [
        # TODO(maia): better solution to wd? (maybe pass in service to each case?)
        # Case('tilt up 1x', test_tilt_up_once, wd=SERVANTES_FE.work_dir),
        # Case('tilt up again, no change', test_tilt_up_again_no_change, wd=SERVANTES_FE.work_dir),
        # Case('tilt up again, new file', test_tilt_up_again_new_file),
        Case('watch build from new file', SERVANTES_FE, test_watch_build_from_new_file),
        Case('watch build from many changed files', SERVANTES_FE, test_watch_build_from_many_changed_files),
        Case('watch build from changed go file', SERVANTES_FE, test_watch_build_from_changed_go_file)
        # Case('tilt up, big file (5MB)', test_tilt_up_big_file),
        # Case('tilt up, new file, checking frontend', test_tilt_up_fe, wd=BLORG_FRONTEND_DIR),
        # Case('watch build, changed file, checking frontend', test_tilt_up_watch_fe, wd=BLORG_FRONTEND_DIR),

        # Leave this commented out unless you particularly want it, it's damn slow.
        # Case('tilt up, REALLY big file (500MB)', test_tilt_up_really_big_file),
    ]

    try:
        for c in cases:
            c.run()

    finally:
        print()
        print(RESULTS_BLOCKLTR)
        env = get_k8s_env()
        print('(Kubernetes environment: {})'.format(env.name))
        print()

        have_results = False
        for c in cases:
            if c.time_seconds:
                have_results = True
                print('\t{} --> {:.5f} seconds'.format(c.name, c.time_seconds))
        if not have_results:
            print('...nvm, no results :(')

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


def curl(url) -> str:
    try:
        out = subprocess.check_output(['curl', url], stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError:
        return ""  # it's ok if the service isn't up yet

    return out.decode('utf-8').strip()


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


def write_fe_token() -> str:
    now = str(datetime.datetime.now())
    call_or_error(['sed', '-e', 's/^.*timing.py.*$/    timing.py {}/'.format(now), '-i', BLORG_FRONTEND_INDEX])
    return now


def wait_for_fe_token(token: str):
    url = fe_url()
    print('Waiting for token to appear: {}'.format(url))

    found = False
    while not found:
        out = curl(url)
        found = token in out
        if not found:
            time.sleep(0.1)


def fe_url():
    env = get_k8s_env()
    if env == K8sEnv.D4M:
        return 'localhost:8081'
    if env == K8sEnv.MINIKUBE:
        me = os.getlogin()
        service = 'devel-{}-lb-blorg-fe'.format(me)
        intervalSec = 1  # 1s is the smallest polling interval we can set :raised_eyebrow:
        out = subprocess.check_output([
            'minikube', 'service', service, '--url', '--interval', intervalSec])
        return out.decode('utf-8').strip()

    raise Exception('Unable to find blorg-fe url')


def clean_up():
    # delete any files we touched
    # TODO(maia): this info should be stored better than in a global var :-/
    # (on its way to deprecation, will store touched files on Service obj's instead)
    global TOUCHED_FILES
    for f in TOUCHED_FILES:
        if os.path.isfile(f):
            os.remove(f)

    for s in SERVICES:
        for f in s.touched_files:
            path = os.path.join(s.write_dir, f)
            if os.path.isfile(path):
                os.remove(path)
        if s.main_go_changed:
            with open(s.main_go_path, 'w') as main_go:
                main_go.write(s.main_go_orig_contents)


### THE TEST CASES
def test_tilt_up_once() -> float:
    # Set-up:
    # mark that tilt up has been called so we can skip setup for later tests
    SERVANTES_FE.up_called = True
    # create a file so we're assured a non-cached image build
    SERVANTES_FE.write_file(KB)

    return time_call(SERVANTES_FE.tilt_up_cmd())


def test_tilt_up_again_no_change() -> float:
    tilt_up_if_not_called(SERVANTES_FE)

    return time_call(SERVANTES_FE.tilt_up_cmd())


def test_tilt_up_again_new_file() -> float:
    tilt_up_if_not_called(SERVANTES_FE)

    SERVANTES_FE.write_file(KB)

    return time_call(SERVANTES_FE.tilt_up_cmd())


def test_tilt_up_fe() -> float:
    call_or_error(tilt_up_fe_cmd)
    token = write_fe_token()
    print('Wrote token "{}", waiting for it to appear in HTML'.format(token))

    with Timer() as t:
        call_or_error(tilt_up_fe_cmd)
        wait_for_fe_token(token)

    return t.duration_secs


def test_tilt_up_watch_fe() -> float:
    tilt_proc = run_and_wait_for_stdout(tilt_up_watch_fe_cmd, '[timing.py] finished initial build')
    token = write_fe_token()
    print('Wrote token "{}", waiting for it to appear in HTML'.format(token))

    with Timer() as t:
        wait_for_fe_token(token)

    tilt_proc.terminate()
    return t.duration_secs


def test_watch_build_from_new_file(serv: Service) -> float:
    # TODO: make sure `tilt up --watch` isn't already running?

    # run `tilt up --watch` and wait for it to finish the initial build
    tilt_proc = run_and_wait_for_stdout(serv.tilt_up_watch_cmd(), '[timing.py] finished initial build')

    # wait a sec for the pod to come up
    time.sleep(1)

    # write a new file (does not affect go build)
    serv.write_file(100 * KB)  # 100KB total

    with Timer() as t:
        wait_for_stdout(tilt_proc, '[timing.py] finished build from file change',
                        kill_on_match=True)
    return t.duration_secs


def test_watch_build_from_many_changed_files(serv: Service) -> float:
    # TODO: make sure `tilt up --watch` isn't already running?

    # run `tilt up --watch` and wait for it to finish the initial build
    tilt_proc = run_and_wait_for_stdout(serv.tilt_up_watch_cmd(), '[timing.py] finished initial build')

    # wait a sec for the pod to come up
    time.sleep(1)

    for _ in range(100):  # 100KB total
        serv.write_file(KB)

    with Timer() as t:
        wait_for_stdout(tilt_proc, '[timing.py] finished build from file change',
                        kill_on_match=True)
    return t.duration_secs


def test_watch_build_from_changed_go_file(serv: Service) -> float:
    # TODO: make sure `tilt up --watch` isn't already running?

    # run `tilt up --watch` and wait for it to finish the initial build
    tilt_proc = run_and_wait_for_stdout(serv.tilt_up_watch_cmd(), '[timing.py] finished initial build')

    # wait a sec for the pod to come up
    time.sleep(1)

    # change a go file
    serv.change_main_go()

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


def tilt_up_if_not_called(serv: Service):
    if serv.up_called:
        print('Initial `tilt up` already called, no setup required')
    else:
        print('Initial call to `tilt up`')
        call_or_error(serv.tilt_up_cmd())
        serv.up_called = True


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
