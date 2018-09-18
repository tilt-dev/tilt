#!/usr/bin/env python

import deploy_utils as utils
from populate_config_template import populate_config_template

import argparse
import getpass
import subprocess
import textwrap


def parse_args():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=textwrap.dedent("""
        Deploy the `synclet` daemonset to Kubernetes development cluster.

        Deploy consists of the following steps:
        1. (re)build Docker image
        2. push docker image to gcr.io
        3. generate synclet config file (by populating template)
        4. create/update synclet from config file
        """))
    parser.add_argument('--config_template', '-c', type=str,
                        help=('path to config template file (default: {})'.format(
                              utils.DEFAULT_TEMPLATE)),
                        default=utils.DEFAULT_TEMPLATE)
    parser.add_argument('--dockerfile', '-d', type=str,
                        help=('path to Dockerfile (default: {})'.format(
                              utils.DEFAULT_DOCKERFILE)),
                        default=utils.DEFAULT_DOCKERFILE)
    return parser.parse_args()


def main():

    # setup
    args = parse_args()
    owner = getpass.getuser()
    imgname = utils.image_name(utils.ENV_DEVEL, owner)

    # TODO(maia): stream output when shelling out

    try:

        # 1. (re)build Docker image
        print('+ (Re)building Docker image...')
        out = subprocess.check_output(['docker', 'build', '-f', args.dockerfile, '-t', imgname, '.'])
        print('~~~ Built Docker image "{}" with output:\n{}'.
            format(imgname, utils.tab_lines(out.decode("utf-8"))))

        # 2. push docker image to gcr.io
        print('+ Pushing Docker image...')
        out = subprocess.check_output(['docker', 'push', imgname])
        print('~~~ Pushed Docker image with output:\n{}'.
            format(utils.tab_lines(out.decode("utf-8"))))

        # 3. generate k8s config file (by populating template)
        print('+ Generating k8s file from template "{}"...'.format(args.config_template))
        config = populate_config_template(args.config_template, utils.ENV_DEVEL, owner)
        print('~~~ Generated config file: "{}"\n'.format(config))

        # 4. create/update k8s from config file
        print('+ Deleting existing pods for this app+owner+env...')
        labels = {
            'app': 'synclet',
            'environment': utils.ENV_DEVEL,
            'owner': owner,
        }
        selectors = []
        for selector in ['{}={}'.format(k, v) for k, v in labels.items()]:
            selectors.append(selector)
        selectors_string = ",".join(selectors)
        cmd = ['kubectl', 'delete', 'pods', '--namespace=kube-system', '-l', selectors_string]
        out = subprocess.check_output(cmd)
        print('~~~ Deleted existing pods (if any) with output:\n{}'.format(
            utils.tab_lines(out.decode("utf-8"))))

        print('+ Applying generated k8s config...')
        out = subprocess.check_output(['kubectl', 'apply', '-f', config])
        print('~~~ Successfully applied config with output:\n{}'.
            format(utils.tab_lines(out.decode("utf-8"))))

    except subprocess.CalledProcessError as e:
        print("failed process output: '%s'" % e.output)
        raise e


if __name__ == '__main__':
    main()
