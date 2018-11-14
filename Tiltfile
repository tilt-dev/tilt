# -*- mode: Python -*-

def test():
  entrypoint = 'cd /go/src/github.com/windmilleng/tilt && make test'
  start_fast_build('Dockerfile.base', 'gcr.io/blorg-dev/tilt-test', entrypoint)
  add(local_git_repo('.'), '/go/src/github.com/windmilleng/tilt')
  run('cd src/github.com/windmilleng/tilt && go build ./...')
  image = stop_build()

  return k8s_service(image, yaml=read_file('tilt-test.yaml'))

def synclet():
  username = local('whoami').rstrip('\n')
  image_tag = 'gcr.io/blorg-dev/synclet:devel-synclet-' + username
  start_fast_build('synclet/Dockerfile.base', image_tag, './server')
  add(local_git_repo('.'), '/go/src/github.com/windmilleng/tilt')
  run('cd /go/src/github.com/windmilleng/tilt && go build -o server ./cmd/synclet/main.go && mv server /app')
  local('synclet/populate_config_template.py devel')
  image = stop_build()

  return k8s_service(image, yaml=read_file('synclet/synclet-conf.generated.yaml'))

def docserver():
  image = static_build('Dockerfile.docserver', 'gcr.io/windmill-public-containers/tilt-docserver')
  s = k8s_service(image, yaml=read_file('docserver.yaml'))
  s.port_forward(10000)
  return s
