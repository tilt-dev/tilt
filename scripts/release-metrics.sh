#!/bin/bash
#
# Usage: ./scripts/release-metrics.sh > releases.csv
#
# Then import the csv into your favorite data analysis program.

echo 'Release,Asset,Download Count,Published at'
curl -s https://api.github.com/repos/tilt-dev/tilt/releases | \
    jq -r '.[] | . as {name:$release,$published_at} | .assets[] | select(.name != "checksums.txt") | [$release,.name,.download_count,$published_at] | @csv'
