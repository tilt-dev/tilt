# -*- mode: Python -*-

k8s_yaml('docserver.yaml')
repo = local_git_repo('.')
img = fast_build('gcr.io/windmill-public-containers/tilt-docserver',
                 'Dockerfile.docserverbase',
                 'cd _build/html && python -m http.server 8000')
img.add(repo.path('docs'), '/src/')
img.run('make html')

k8s_resource('tilt-docserver', port_forwards=10000)
