package configmap

// Functions for parsing out common ConfigMaps.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TriggerQueue(ctx context.Context, client client.Client) (*v1alpha1.ConfigMap, error) {
	var cm v1alpha1.ConfigMap
	err := client.Get(ctx, types.NamespacedName{Name: tiltfiles.TriggerQueueConfigMapName}, &cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	return &cm, nil
}

func InTriggerQueue(cm *v1alpha1.ConfigMap, nn types.NamespacedName) bool {
	name := nn.Name
	for k, v := range cm.Data {
		if !strings.HasSuffix(k, "-name") {
			continue
		}

		if v == name {
			return true
		}
	}
	return false
}

func TriggerQueueReason(cm *v1alpha1.ConfigMap, nn types.NamespacedName) model.BuildReason {
	name := nn.Name
	for k, v := range cm.Data {
		if !strings.HasSuffix(k, "-name") {
			continue
		}

		if v != name {
			continue
		}

		cur := strings.TrimSuffix(k, "-name")
		reasonCode := cm.Data[fmt.Sprintf("%s-reason-code", cur)]
		i, err := strconv.Atoi(reasonCode)
		if err != nil {
			return model.BuildReasonFlagTriggerUnknown
		}
		return model.BuildReason(i)
	}
	return model.BuildReasonNone
}
