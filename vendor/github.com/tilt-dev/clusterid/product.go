package clusterid

import (
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Enum of possible values for cluster.product
//
// Named in honor of the product component of the user-agent string,
// which we hope isn't foreshadowing.
type Product string

func (p Product) DefaultClusterName() string {
	if p == ProductKIND {
		return "kind-kind"
	}
	if p == ProductK3D {
		return "k3d-k3s-default"
	}
	return string(p)
}

func (p Product) String() string { return string(p) }

const (
	ProductUnknown        Product = "unknown"
	ProductGKE            Product = "gke"
	ProductMinikube       Product = "minikube"
	ProductDockerDesktop  Product = "docker-desktop"
	ProductMicroK8s       Product = "microk8s"
	ProductCRC            Product = "crc"
	ProductKrucible       Product = "krucible"
	ProductKIND           Product = "kind"
	ProductK3D            Product = "k3d"
	ProductRancherDesktop Product = "rancher-desktop"
	ProductColima         Product = "colima"
)

func (p Product) IsDevCluster() bool {
	return p == ProductMinikube ||
		p == ProductDockerDesktop ||
		p == ProductMicroK8s ||
		p == ProductCRC ||
		p == ProductKIND ||
		p == ProductK3D ||
		p == ProductKrucible ||
		p == ProductRancherDesktop ||
		p == ProductColima
}

func ProductFromContext(c *clientcmdapi.Context, cl *clientcmdapi.Cluster) Product {
	cn := c.Cluster
	if strings.HasPrefix(cn, string(ProductMinikube)) {
		return ProductMinikube
	} else if strings.HasPrefix(cn, "docker-for-desktop-cluster") || strings.HasPrefix(cn, "docker-desktop") {
		return ProductDockerDesktop
	} else if strings.HasPrefix(cn, string(ProductGKE)) {
		// GKE cluster strings look like:
		// gke_blorg-dev_us-central1-b_blorg
		return ProductGKE
	} else if cn == "kind" {
		return ProductKIND
	} else if strings.HasPrefix(cn, "kind-") {
		// As of KinD 0.6.0, KinD uses a context name prefix
		// https://github.com/kubernetes-sigs/kind/issues/1060
		return ProductKIND
	} else if strings.HasPrefix(cn, "microk8s-cluster") {
		return ProductMicroK8s
	} else if strings.HasPrefix(cn, "api-crc-testing") {
		return ProductCRC
	} else if strings.HasPrefix(cn, "krucible-") {
		return ProductKrucible
	} else if strings.HasPrefix(cn, "k3d-") {
		return ProductK3D
	} else if strings.HasPrefix(cn, "rancher-desktop") {
		return ProductRancherDesktop
	} else if strings.HasPrefix(cn, "colima") {
		return ProductColima
	}

	loc := c.LocationOfOrigin
	homedir, err := homedir.Dir()
	if err != nil {
		return ProductUnknown
	}

	k3dDir := filepath.Join(homedir, ".config", "k3d")
	if strings.HasPrefix(loc, k3dDir+string(filepath.Separator)) {
		return ProductK3D
	}

	minikubeDir := filepath.Join(homedir, ".minikube")
	if cl != nil && cl.CertificateAuthority != "" &&
		strings.HasPrefix(cl.CertificateAuthority, minikubeDir+string(filepath.Separator)) {
		return ProductMinikube
	}

	// NOTE(nick): Users can set the KIND cluster name with `kind create cluster
	// --name`.  This makes the KIND cluster really hard to detect.
	//
	// We currently do it by assuming that KIND configs are always stored in a
	// file named kind-config-*.
	//
	// KIND internally looks for its clusters with `docker ps` + filters,
	// which might be a route to explore if this isn't robust enough.
	//
	// This is for old pre-0.6.0 versions of KinD
	if strings.HasPrefix(filepath.Base(loc), "kind-config-") {
		return ProductKIND
	}

	return ProductUnknown
}
