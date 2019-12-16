// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace

import (
	"context"

	"go.opentelemetry.io/otel/api/core"
	apitrace "go.opentelemetry.io/otel/api/trace"
)

type tracer struct {
	provider *Provider
	name     string
}

var _ apitrace.Tracer = &tracer{}

func (tr *tracer) Start(ctx context.Context, name string, o ...apitrace.SpanOption) (context.Context, apitrace.Span) {
	var opts apitrace.SpanOptions
	var parent core.SpanContext
	var remoteParent bool

	//TODO [rghetia] : Add new option for parent. If parent is configured then use that parent.
	for _, op := range o {
		op(&opts)
	}

	if relation := opts.Relation; relation.SpanContext != core.EmptySpanContext() {
		switch relation.RelationshipType {
		case apitrace.ChildOfRelationship, apitrace.FollowsFromRelationship:
			parent = relation.SpanContext
			remoteParent = true
		default:
			// Future relationship types may have different behavior,
			// e.g., adding a `Link` instead of setting the `parent`
		}
	} else {
		if p := apitrace.CurrentSpan(ctx); p != nil {
			if sdkSpan, ok := p.(*span); ok {
				sdkSpan.addChild()
				parent = sdkSpan.spanContext
			}
		}
	}

	spanName := tr.spanNameWithPrefix(name)
	span := startSpanInternal(tr, spanName, parent, remoteParent, opts)
	for _, l := range opts.Links {
		span.addLink(l)
	}
	span.SetAttributes(opts.Attributes...)

	span.tracer = tr

	if span.IsRecording() {
		sps, _ := tr.provider.spanProcessors.Load().(spanProcessorMap)
		for sp := range sps {
			sp.OnStart(span.data)
		}
	}

	ctx, end := startExecutionTracerTask(ctx, spanName)
	span.executionTracerTaskEnd = end
	return apitrace.SetCurrentSpan(ctx, span), span
}

func (tr *tracer) WithSpan(ctx context.Context, name string, body func(ctx context.Context) error) error {
	ctx, span := tr.Start(ctx, name)
	defer span.End()

	if err := body(ctx); err != nil {
		// TODO: set event with boolean attribute for error.
		return err
	}
	return nil
}

func (tr *tracer) spanNameWithPrefix(name string) string {
	if tr.name != "" {
		return tr.name + "/" + name
	}
	return name
}
