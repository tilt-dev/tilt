# -*- mode: Python -*-
enable_feature("team_alerts")

k8s_resource_assembly_version(2)
k8s_yaml('deployments/sail.yaml')

repo = local_git_repo('.')
live_update = [
  sync('internal', '/go/src/github.com/windmilleng/tilt/internal'),
  sync('web', '/go/src/github.com/windmilleng/tilt/web'),
  run('make build-js', trigger=['web/']),
  run('make install-sail', trigger=['internal/']),
  restart_container(),
]
docker_build('gcr.io/windmill-public-containers/sail',
             '.',
             dockerfile='deployments/sail.dockerfile',
             live_update=live_update)

k8s_resource('sail', port_forwards=10450)
