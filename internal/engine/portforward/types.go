package portforward

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type PortForward = v1alpha1.PortForward
type PortForwardSpec = v1alpha1.PortForwardSpec
type PortForwardStatus = v1alpha1.PortForwardStatus
type ObjectMeta = metav1.ObjectMeta
type Forward = v1alpha1.Forward
