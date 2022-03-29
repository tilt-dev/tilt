#!/bin/sh

set -ex

cp compiled.txt index.html
exec httpd -f -p 8000
