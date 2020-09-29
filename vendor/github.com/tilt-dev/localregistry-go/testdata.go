package localregistry

// Copied verbatim from
// https://github.com/kubernetes/enhancements/blob/0d69f7cea6fbe73a7d70fab569c6898f5ccb7be0/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry/README.md

const SampleConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5000"
    hostFromContainerRuntime: "registry:5000"
    hostFromClusterNetwork: "kind-registry:5000"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
`
