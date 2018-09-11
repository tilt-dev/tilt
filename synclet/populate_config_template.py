#!/usr/bin/env python

import deploy_utils as utils

import argparse
import getpass

KEY_ENVIRONMENT = 'environment'
KEY_OWNER = 'owner'
KEY_IMAGE_NAME = 'imgname'
KEY_BACKEND_ADDR = 'backend_addr'

def parse_args():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description='Populate k8s config file template with the appropriate values.')
    parser.add_argument('environment', type=str,
                        help='environment you\'re deploying to',
                        choices=[utils.ENV_DEVEL, utils.ENV_PROD])
    parser.add_argument('--file', type=str,
                        help=('path to template file (default: %s)' %
                              utils.DEFAULT_TEMPLATE),
                        default=utils.DEFAULT_TEMPLATE)
    return parser.parse_args()


def get_file(filename):
    with open(filename) as infile:
        body = infile.read()

    return body


def write_file(filename, contents):
    """Write the given contents to the given file. (If file exists, overwrite it.)"""
    with open(filename, 'w') as outfile:
        outfile.write(contents)


def outfile_name(infile):
    """Given infile (i.e. template file), generate outfile name."""
    outfile = '%s.%s' % (infile, 'generated')
    if 'template' in infile:
        outfile = infile.replace('template', 'generated')
    return outfile


def populate_config_template(infile, env, owner):
    """
    Populate config template (`infile`) with the given environment and owner.
    Return the path of the generated config file.
    """
    temp_vals = {
        KEY_ENVIRONMENT: env,
        KEY_OWNER: owner,
        KEY_IMAGE_NAME: utils.image_name(env, owner),
    }
    template = get_file(infile)

    outfile = outfile_name(infile)

    populated = template % temp_vals

    write_file(outfile, populated)

    return outfile


def main():
    args = parse_args()
    owner = getpass.getuser()

    outfile = populate_config_template(args.file, args.environment, owner)
    print('Successfully generated config file: "%s"' % outfile)


if __name__ == '__main__':
    main()
