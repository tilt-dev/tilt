package fake

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// Wrap the fake client in other assertions we want to make.
type fakeTiltClient struct {
	ctrlclient.Client
}

func (f fakeTiltClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	forCreate := obj.DeepCopyObject().(client.Object)
	if defaulter, ok := forCreate.(resourcestrategy.Defaulter); ok {
		defaulter.Default()
	}

	if err := f.Client.Create(ctx, forCreate, opts...); err != nil {
		return err
	}

	return simulateMarshalRoundtrip(forCreate, obj)
}

func (f fakeTiltClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		// controller-runtime fake ignores context; check it here to allow controllers to test
		// handling of cancellation related errors
		return ctxErr
	}

	forUpdate := obj.DeepCopyObject().(client.Object)
	if defaulter, ok := forUpdate.(resourcestrategy.Defaulter); ok {
		defaulter.Default()
	}

	err := f.Client.Update(ctx, forUpdate, opts...)
	if err != nil {
		return err
	}

	return simulateMarshalRoundtrip(forUpdate, obj)
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

// simulateMarshalRoundtrip makes API types behave closer to reality for the
// fake client.
//
// NOTE(nick): We've had a lot of bugs due to the way the apiserver
// modifies and truncates objects on update. We simulate this behavior
// to catch this class of bug.
//
// The `apiResp` argument is marshaled and then unmarshalled into `dest` arg.
// (This allows the fake client to mutate a *copy* of the user-provided obj, and
// only mutate the *original* obj here on success.)
func simulateMarshalRoundtrip(apiResp, dest ctrlclient.Object) error {
	content, err := json.Marshal(apiResp)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, dest); err != nil {
		return err
	}
	return nil
}
