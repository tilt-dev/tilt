#!/bin/sh

set -ex

PORT=$1
cp compiled.txt index.html
exec busybox httpd -f -p $PORT
