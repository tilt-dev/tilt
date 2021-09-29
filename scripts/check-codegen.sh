#!/usr/bin/env bash

# This script ensures that codegen files are up-to-date based on changes to HEAD as
# compared to origin/master.
#
# Because the codegen script is pretty slow, it bails early if no files in pkg/apis
# (or pkg/openapi) were modified.
#
# If there were changed files, the codegen script is run and if there are any uncommitted
# files as a result, it fails with an error.

set -e

dir=$(dirname "$0")
cd "${dir}/.."

codegen_regex="pkg/apis|pkg/openapi"

function print_file_list() {
    while IFS= read -r line
    do
      if [[ $line =~ $codegen_regex ]]; then
        printf "  - %s\n" "${line}"
      fi
    done < <(printf '%s\n' "${*}")
}

master_sha=$(git rev-parse origin/master)
changes=$(git diff-tree --no-commit-id --no-renames --name-only -r "$(git merge-base "${master_sha}" HEAD)" HEAD)
if [[ $changes =~ $codegen_regex ]]; then
  echo "Found changed API files (compared to origin/master):"
  print_file_list "${changes}"
  printf "\nRunning codegen to ensure up-to-date...\n\n"
  make update-codegen
else
  echo "No API files modified (skipping up-to-date check)"
  exit 0
fi

# find any uncommitted changes: getting a list of modified (staged + unstaged) as well as
# untracked is really only doable with git status; the porcelain format is stable and has
# a fixed length prefix for each line that can be chopped off to just get the filenames
modified=$(git status --porcelain --no-renames | cut -c 4-)
if [[ $modified =~ $codegen_regex ]]; then
  >&2 echo "Found out of sync codegen files:"
  >&2 print_file_list "${modified}"
  if [[ -n "${CIRCLECI}" ]]; then
    >&2 printf "\nRun make update-codegen locally and push the changes.\n"
  else
    >&2 printf "\nThe modified files should be committed before pushing.\n"
  fi
  exit 1
fi

echo "All codegen files up to date!"
