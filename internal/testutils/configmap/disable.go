package configmap

import (
	"context"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func UpsertDisableConfigMap(ctx context.Context, client ctrlclient.Client, name string, key string, isDisabled bool) error {
	cm := &v1alpha1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, client, cm, func() error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[key] = strconv.FormatBool(isDisabled)
		return nil
	})
	return err
}
