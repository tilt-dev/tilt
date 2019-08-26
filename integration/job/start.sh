#!/bin/sh

echo "Starting busybox on port 8000"
busybox httpd -f -p 8000
