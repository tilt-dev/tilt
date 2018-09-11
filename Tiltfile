# -*- mode: Python -*-

def test():
  entrypoint = 'cd /go/src/github.com/windmilleng/tilt && make test'
  image = build_docker_image('Dockerfile.base', 'gcr.io/blorg-dev/tilt-test', entrypoint)
  image.add(local_git_repo('.'), '/go/src/github.com/windmilleng/tilt')
  image.run('cd src/github.com/windmilleng/tilt && go build ./...')
  return k8s_service(read_file('tilt-test.yaml'), image)
