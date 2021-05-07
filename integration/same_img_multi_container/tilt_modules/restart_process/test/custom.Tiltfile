load('../Tiltfile', 'custom_build_with_restart')

k8s_yaml('job.yaml')
custom_build_with_restart(
  'failing_job',
  command='docker build -t $EXPECTED_REF .',
  deps=['fail.sh'],
  entrypoint='/fail.sh',
  live_update=[sync('./fail.sh', '/fail.sh')])
