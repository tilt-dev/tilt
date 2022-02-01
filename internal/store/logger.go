package store

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
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
func MustObjectLogHandler(ctx context.Context, st Dispatcher, obj metav1.Object) context.Context {
	ctx, err := WithObjectLogHandler(ctx, st, obj)
	if err != nil {
		panic(err)
	}
	return ctx
}

// Read labels and annotations of the given API object to determine where to log.
func WithObjectLogHandler(ctx context.Context, st Dispatcher, obj metav1.Object) (context.Context, error) {
	// It's ok if the manifest or span id don't exist, they will just
	// get dumped in the global log.
	mn := obj.GetAnnotations()[v1alpha1.AnnotationManifest]
	spanID := obj.GetAnnotations()[v1alpha1.AnnotationSpanID]
	typ, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, fmt.Errorf("object missing type data: %T", obj)
	}
	if spanID == "" {
		spanID = fmt.Sprintf("%s-%s", typ.GetKind(), obj.GetName())
	}

	return WithManifestLogHandler(ctx, st, model.ManifestName(mn), model.LogSpanID(spanID)), nil
}

func WithManifestLogHandler(ctx context.Context, st Dispatcher, mn model.ManifestName, spanID logstore.SpanID) context.Context {
	w := manifestLogWriter{
		store:        st,
		manifestName: mn,
		spanID:       spanID,
	}
	return logger.CtxWithLogHandler(ctx, w)
}

type manifestLogWriter struct {
	store        Dispatcher
	manifestName model.ManifestName
	spanID       model.LogSpanID
}

func (w manifestLogWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(NewLogAction(w.manifestName, w.spanID, level, fields, p))
	return nil
}

type Dispatcher interface {
	Dispatch(action Action)
}
