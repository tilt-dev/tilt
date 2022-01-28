package tiltfiles

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func SetTiltfileArgs(ctx context.Context, client client.Client, args []string) error {
	nn := types.NamespacedName{Name: model.MainTiltfileManifestName.String()}
	var tf v1alpha1.Tiltfile
	err := client.Get(ctx, nn, &tf)
	if err != nil {
		return err
	}

	if apicmp.DeepEqual(tf.Spec.Args, args) {
		return nil
	}

	update := tf.DeepCopy()
	update.Spec.Args = args
	err = client.Update(ctx, update)
	if err != nil {
		return err
	}

	return nil
}
