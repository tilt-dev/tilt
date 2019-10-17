package k8s

// An attempt to order things to help k8s, e.g.
// a Service should come before things that refer to it.
// Namespace should be first.
// In some cases order just specified to provide determinism.
// Borrowed from Kustomize: https://github.com/kubernetes-sigs/kustomize/blob/b878cd050d5ffe5d485b34b496c915fa96405db9/pkg/gvk/gvk.go#L82
var orderFirst = []string{
	"Namespace",
	"StorageClass",
	"CustomResourceDefinition",
	"MutatingWebhookConfiguration",
	"ServiceAccount",
	"PodSecurityPolicy",
	"Role",
	"ClusterRole",
	"RoleBinding",
	"ClusterRoleBinding",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"ConfigMap",
	"Secret",
	"Service",
	"LimitRange",
	"Deployment",
	"StatefulSet",
	"CronJob",
	"PodDisruptionBudget",
}

var orderFirstIndexMap = func() map[string]int {
	indexMap := make(map[string]int, len(orderFirst))
	for i, kind := range orderFirst {
		indexMap[kind] = i
	}
	return indexMap
}()

type entityList []K8sEntity

func (l entityList) Len() int { return len(l) }
func (l entityList) Less(i, j int) bool {
	kindI := i + 1000
	kindJ := j + 1000
	if ind, ok := orderFirstIndexMap[l[i].GVK().Kind]; ok {
		kindI = ind
	}
	if ind, ok := orderFirstIndexMap[l[j].GVK().Kind]; ok {
		kindJ = ind
	}
	return kindI < kindJ
}
func (l entityList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
