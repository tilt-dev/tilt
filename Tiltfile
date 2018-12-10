# -*- mode: Python -*-

def get_username():
  return str(local('whoami')).rstrip('\n')

# docserver
k8s_resource('docserver', 'docserver.yaml', port_forwards=10000)
docker_build('gcr.io/windmill-public-containers/tilt-docserver', '.', dockerfile='Dockerfile.docserver')

# test
repo = local_git_repo('.')
ep = 'cd /go/src/github.com/windmilleng/tilt && make test'
test_img = (fast_build('gcr.io/blorg-dev/tilt-test', 'Dockerfile.base', entrypoint=ep)
  .add(repo, '/go/src/github.com/windmilleng/tilt')
  .run('cd src/github.com/windmilleng/tilt && go build ./...'))

k8s_resource('test', yaml='tilt-test.yaml', image=test_img)

# synclet
image_tag = 'gcr.io/blorg-dev/synclet:devel-synclet-' + get_username()

synclet_img = (fast_build(image_tag, 'synclet/Dockerfile.base', entrypoint='./server')
  .add(repo, '/go/src/github.com/windmilleng/tilt')
  .run('cd /go/src/github.com/windmilleng/tilt && go build -o server ./cmd/synclet/main.go && mv server /app'))

local('synclet/populate_config_template.py devel')
k8s_resource('synclet', yaml='synclet/synclet-conf.generated.yaml', image=synclet_img)
