def remove_all_empty_and_whitespace(my_list):
  ret = []
  for x in my_list:
    s = x.strip()
    if s:
      ret.append(s)

  return ret

 #TODO(dmiller): can I memoize this or something?
def get_all_go_files(path):
  return str(local('cd %s && find . -type f -name "*.go" | grep -v vendor' % path)).split("\n")

def get_deps_for_pkg(pkg):
  cmd = """go list -f '{{ join .GoFiles "\\n" }} {{ join .TestGoFiles "\\n" }} {{ join .XTestGoFiles "\\n"}}' %s""" % pkg
  split = str(local(cmd)).split("\n")
  return remove_all_empty_and_whitespace(split)

def go(name, entrypoint, srv=""):
  all_packages = remove_all_empty_and_whitespace(str(local('go list ./...')).split("\n"))
  # for pkg in all_packages:
  #   pkg_deps = get_deps_for_pkg(pkg)
  #   local_resource("go_test_%s" % pkg, "go test %s" % pkg, deps=pkg_deps)

  local_resource(name, "go build -o /tmp/%s %s" % (name, entrypoint), serve_cmd=srv, deps=get_all_go_files("."))

def go_lint():
  local_resource("go_lint", "make lint", deps=get_all_go_files("."))

def get_all_ts_files(path):
  return str(local('cd %s && find . -type f -name "*.ts*" | grep -v node_modules | grep -v __snapshots__' % path)).split("\n")

def yarn_install():
  local_resource("yarn_install", "cd web && yarn")

def jest(path):
  local_resource("web_jest", "", serve_cmd="cd %s && yarn test" % path, resource_deps=["yarn_install"])

def web_lint():
  ts_deps = get_all_ts_files("web")
  local_resource("web_lint", "cd web && yarn run check", deps=ts_deps, resource_deps=["yarn_install"])

go("Tilt", "cmd/tilt/main.go", srv="cd /tmp/ && ./tilt up --hud=false --no-browser --web-mode=prod --port=9765")

yarn_install()
jest("web")
web_lint()
go_lint()
