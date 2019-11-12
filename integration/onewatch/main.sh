#!/bin/sh

set -ex

cp compiled.txt index.html
exec busybox httpd -f -p 8000
