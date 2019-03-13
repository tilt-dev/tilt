#!/usr/bin/python
#
# Usage:
# scripts/upload-assets.py VERSION
#
# where VERSION is a version string like "v1.2.3" or "latest"
#
# Generates static assets for HTML, JS, and CSS.
# Then uploads them to the public bucket.

import argparse
import os
import re
import subprocess
import sys

parser = argparse.ArgumentParser(description='Upload JS assets')
parser.add_argument('--force', dest='feature', action='store_true',
                    help='Re-upload files even if they already exist')
parser.add_argument('version', help='A version string like "v1.2.3".')
args = parser.parse_args()
version = args.version
if version == "latest":
  version = str(subprocess.check_output([
    "git", "describe", "--tags", "--abbrev=0"
  ])).strip()

if not re.match('^v[0-9]+[.][0-9]+[.][0-9]+', version):
  print("Usage: scripts/upload-assets.py VERSION")
  sys.exit(1)

dir_url = ("https://storage.googleapis.com/tilt-static-assets/%s/" % version)
url = dir_url + "index.html"
print("Uploading to %s" % dir_url)
status = subprocess.call([
  "gsutil", "stat", "gs://tilt-static-assets/%s/index.html" % version
])
if status == 0:
  print("Error: Files already exist: %s" % url)
  sys.exit(1)

os.chdir("web")
os.system('yarn install')
os.system('yarn run build')
os.system('gsutil cp -r build "gs://tilt-static-assets/%s"' % version)
