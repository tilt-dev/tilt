#!/bin/sh

PACKAGE=$1
FILES=$(gofmt -l $PACKAGE)
if [ "x$FILES" != "x" ]; then
    echo "Files with gofmt errors:"
    echo "$FILES"
    exit 1
fi
