package store

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func NewLogActionLogger(ctx context.Context, dispatch func(action Action)) logger.Logger {
	l := logger.Get(ctx)
	return logger.NewFuncLogger(l.SupportsColor(), l.Level(), func(level logger.Level, fields logger.Fields, b []byte) error {
		dispatch(NewGlobalLogAction(level, b))
		return nil
	})
}

// Read labels and annotations of the given API object to determine where to log,
// panicking if there's no info available.
func MustObjectLogHandler(ctx context.Context, st RStore, obj runtime.Object) context.Context {
	ctx, err := WithObjectLogHandler(ctx, st, obj)
	if err != nil {
		panic(err)
	}
	return ctx
}

// Read labels and annotations of the given API object to determine where to log.
func WithObjectLogHandler(ctx context.Context, st RStore, obj runtime.Object) (context.Context, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("object missing log info: %T", obj)
	}

	mn := meta.GetLabels()[v1alpha1.LabelManifest]
	if mn == "" {
		return nil, fmt.Errorf("object missing manifest label")
	}

	spanID := meta.GetAnnotations()[v1alpha1.AnnotationSpanID]
	if spanID == "" {
		return nil, fmt.Errorf("object missing span id annotation")
	}

	w := apiLogWriter{
		store:        st,
		manifestName: model.ManifestName(mn),
		spanID:       model.LogSpanID(spanID),
	}
	return logger.CtxWithLogHandler(ctx, w), nil
}

type apiLogWriter struct {
	store        RStore
	manifestName model.ManifestName
	spanID       model.LogSpanID
}

func (w apiLogWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(NewLogAction(w.manifestName, w.spanID, level, fields, p))
	return nil
}
