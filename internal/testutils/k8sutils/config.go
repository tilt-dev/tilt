package k8sutils

import "k8s.io/client-go/tools/clientcmd/api"

func NewConfig(contextName string, clusterName string, server string) *api.Config {
	ret := api.NewConfig()
	ret.CurrentContext = contextName
	context := api.NewContext()
	context.Cluster = clusterName
	ret.Contexts[contextName] = context
	cluster := api.NewCluster()
	cluster.Server = server
	ret.Clusters[clusterName] = cluster
	return ret
}
