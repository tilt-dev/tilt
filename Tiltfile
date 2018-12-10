# -*- mode: Python -*-

k8s_yaml('docserver.yaml')
docker_build('gcr.io/windmill-public-containers/tilt-docserver', '.',
             dockerfile='Dockerfile.docserver')
k8s_resource('tilt-docserver', port_forwards=10000)
