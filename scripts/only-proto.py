#!/usr/bin/python

import subprocess

gitdiff = subprocess.Popen(['git', 'diff', '--name-only', 'master', 'HEAD'], stderr=subprocess.PIPE, stdout=subprocess.PIPE, universal_newlines=True)
results = gitdiff.communicate()
filtered_results = [k for k in results if k.isspace()]
proto_changes = [k for k in filtered_results if '.proto' or '.pb.go' in k]
len_changes = len(filtered_results)
len_proto_changes = len(proto_changes)

if len_proto_changes > 0 and len_proto_changes != len_changes:
  print("Changes must consist of either no proto changes, or only proto changes")
  print("Saw these changes files:")
  print(filtered_changes)
  exit(1)
