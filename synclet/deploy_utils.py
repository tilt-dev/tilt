# CONSTANTS
DEFAULT_TEMPLATE = 'synclet/synclet-conf.template.yaml'
DEFAULT_DOCKERFILE = 'Dockerfile.synclet'

ENV_DEVEL = 'devel'
ENV_PROD = 'prod'

ENV_TO_PROJ = {
    ENV_DEVEL: 'blorg-dev',  # TODO(maia): new cluster??
    ENV_PROD: 'blorg-prod'   # probably? idk!
}


def docker_tag(env, owner):
    return '{}-synclet-{}'.format(env, owner)


def image_name(env, owner):
    """Generate the canonical name of the docker image for this server+env+user."""
    server = 'synclet'
    gcloud_proj = ENV_TO_PROJ[env]
    tag = docker_tag(env, owner)

    return 'gcr.io/{gcloud_proj}/{server}:{tag}'.format(
        gcloud_proj=gcloud_proj,
        server=server,
        tag=tag,
    )


def tab_lines(s):
    lines = s.split('\n')
    lines[0] = '\t' + lines[0]
    return '\n\t'.join(lines)

