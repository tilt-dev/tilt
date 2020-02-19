def remove_all_empty_and_whitespace(my_list):
  ret = []
  for x in my_list:
    s = x.strip()
    if s:
      ret.append(s)

  return ret

 #TODO(dmiller): can I memoize this or something?
def get_all_go_files(path):
  return str(local('cd %s && find . -type f -name "*.go"' % path)).split("\n")

def get_deps_for_pkg(pkg):
  cmd = """go list -f '{{ join .GoFiles "\\n" }} {{ join .TestGoFiles "\\n" }} {{ join .XTestGoFiles "\\n"}}' '%s'""" % pkg
  split = str(local(cmd)).split("\n")
  return remove_all_empty_and_whitespace(split)

def go(name, entrypoint, all_go_files, srv=""):
  all_packages = remove_all_empty_and_whitespace(str(local('go list ./...')).rstrip().split("\n"))
  # for pkg in all_packages:
  #   pkg_deps = get_deps_for_pkg(pkg)
  #   local_resource("go_test_%s" % pkg, "go test %s" % pkg, deps=pkg_deps)

  local_resource(name, "go build -o /tmp/%s %s" % (name, entrypoint), serve_cmd=srv, deps=all_go_files)

def go_lint(all_go_files):
  local_resource("go_lint", "make lint", deps=all_go_files)

def get_all_ts_files(path):
  res = str(local('cd %s && find . -type f -name "*.ts*" | grep -v node_modules | grep -v __snapshots__' % path)).rstrip().split("\n")
  return res

def yarn_install():
  local_resource("yarn_install", "cd web && yarn", deps=['web/package.json', 'web/yarn.lock'])

def jest(path):
  local_resource("web_jest", serve_cmd="cd %s && yarn run test --notify " % path, resource_deps=["yarn_install"])

def web_lint():
  ts_deps = get_all_ts_files("web")
  local_resource("web_lint", "cd web && yarn run check", deps=ts_deps, resource_deps=["yarn_install"])

def go_vendor():
  local_resource("go_vendor", "make vendor", deps=['go.sum', 'go.mod'])

all_go_files = get_all_go_files(".")

go("Tilt", "cmd/tilt/main.go", all_go_files, srv="cd /tmp/ && ./tilt up --hud=false --no-browser --web-mode=prod --port=9765")
go_lint(all_go_files)
go_vendor()

yarn_install()
jest("web")
web_lint()
