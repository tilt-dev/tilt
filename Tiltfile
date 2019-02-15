# -*- mode: Python -*-

# A fun dogfooding exercise where we run the Tilt tests as jobs on Kubernetes
k8s_yaml('deployments/test.yaml')

repo = local_git_repo('.')

local('echo hi')

def test_job(name):
  img = fast_build(
    'gcr.io/windmill-test-containers/tilt-%s-test' % name,
    'test-base.dockerfile',
    'go test github.com/windmilleng/tilt/internal/%s/...' % name,
    cache = [
      '/root/.cache/go-build',
    ])
  img.add(repo.path('.'), '/go/src/github.com/windmilleng/tilt')
  img.run('go build github.com/windmilleng/tilt/internal/%s/...' % name)

test_job('k8s')
test_job('engine')
