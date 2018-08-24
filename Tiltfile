# -*- mode: Python -*-

def test():
  entrypoint = 'cd /go/src/github.com/windmilleng/tilt && make test'
  image = build_docker_image('Dockerfile.base', 'gcr.io/blorg-dev/tilt-test', entrypoint)
  image.add('/go/src/github.com/windmilleng/tilt', local_git_repo('.'))
  image.run('cd src/github.com/windmilleng/tilt && go build ./...')
  return k8s_service(read_file('tilt-test.yaml'), image)

def read_file(filepath):
  return local("cat " + filepath)
