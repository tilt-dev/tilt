package servicebuilder

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ServiceBuilder struct {
	t        testing.TB
	manifest model.Manifest

	uid      types.UID
	port     int32
	nodePort int32
	ip       string
}

func New(t testing.TB, manifest model.Manifest) ServiceBuilder {
	return ServiceBuilder{
		t:        t,
		manifest: manifest,
	}
}

func (sb ServiceBuilder) WithUID(uid types.UID) ServiceBuilder {
	sb.uid = uid
	return sb
}

func (sb ServiceBuilder) WithPort(port int32) ServiceBuilder {
	sb.port = port
	return sb
}

func (sb ServiceBuilder) WithNodePort(port int32) ServiceBuilder {
	sb.nodePort = port
	return sb
}

func (sb ServiceBuilder) WithIP(ip string) ServiceBuilder {
	sb.ip = ip
	return sb
}

func (sb ServiceBuilder) name() string {
	return fmt.Sprintf("%s-service", sb.manifest.Name)
}

func (sb ServiceBuilder) getUid() types.UID {
	if sb.uid != "" {
		return sb.uid
	}
	return types.UID(fmt.Sprintf("%s-uid", sb.name()))
}

func (sb ServiceBuilder) Build() *v1.Service {
	ports := []v1.ServicePort{}
	if sb.port != 0 || sb.nodePort != 0 {
		ports = append(ports, v1.ServicePort{Port: sb.port, NodePort: sb.nodePort})
	}
	ingress := []v1.LoadBalancerIngress{}
	if sb.ip != "" {
		ingress = append(ingress, v1.LoadBalancerIngress{IP: sb.ip})
	}

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   sb.name(),
			Labels: k8s.NewTiltLabelMap(),
			UID:    sb.getUid(),
		},
		Spec: v1.ServiceSpec{Ports: ports},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{Ingress: ingress},
		},
	}
}
