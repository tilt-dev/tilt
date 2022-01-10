package fake

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Wrap the fake client in other assertions we want to make.
type fakeTiltClient struct {
	ctrlclient.Client
}

// NOTE(nick): We've had a lot of bugs due to the way the apiserver
// modifies and truncates objects on update.
//
// We simulate this behavior to catch this class of bug.
func (f fakeTiltClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		// controller-runtime fake ignores context; check it here to allow controllers to test
		// handling of cancellation related errors
		return ctxErr
	}

	err := f.Client.Update(ctx, obj, opts...)
	if err != nil {
		return err
	}

	content, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, obj)
}

func (f fakeTiltClient) Status() client.StatusWriter {
	return fakeStatusWriter{f.Client.Status()}
}

type fakeStatusWriter struct {
	ctrlclient.StatusWriter
}

func (f fakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		// controller-runtime fake ignores context; check it here to allow controllers to test
		// handling of cancellation related errors
		return ctxErr
	}

	err := f.StatusWriter.Update(ctx, obj, opts...)
	if err != nil {
		return err
	}

	content, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, obj)
}

func (f fakeStatusWriter) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		// controller-runtime fake ignores context; check it here to allow controllers to test
		// handling of cancellation related errors
		return ctxErr
	}

	err := f.StatusWriter.Patch(ctx, obj, patch, opts...)
	if err != nil {
		return err
	}

	content, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, obj)
}
