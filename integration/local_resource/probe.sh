#!/bin/bash

if [[ ! -f ./probe-success ]]; then
  # failure -> stderr
  echo "fake probe failure message" 1>&2
  exit 1
fi

# success -> stdout
echo "fake probe success message"
exit 0
