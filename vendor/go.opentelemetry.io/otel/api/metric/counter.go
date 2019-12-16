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

package metric

import (
	"context"

	"go.opentelemetry.io/otel/api/core"
)

// Float64Counter is a metric that accumulates float64 values.
type Float64Counter struct {
	commonMetric
}

// Int64Counter is a metric that accumulates int64 values.
type Int64Counter struct {
	commonMetric
}

// Float64CounterHandle is a handle for Float64Counter.
//
// It inherits the Release function from commonHandle.
type Float64CounterHandle struct {
	commonHandle
}

// Int64CounterHandle is a handle for Int64Counter.
//
// It inherits the Release function from commonHandle.
type Int64CounterHandle struct {
	commonHandle
}

// AcquireHandle creates a handle for this counter. The labels should
// contain the keys and values for each key specified in the counter
// with the WithKeys option.
//
// If the labels do not contain a value for the key specified in the
// counter with the WithKeys option, then the missing value will be
// treated as unspecified.
func (c *Float64Counter) AcquireHandle(labels LabelSet) (h Float64CounterHandle) {
	h.commonHandle = c.acquireCommonHandle(labels)
	return
}

// AcquireHandle creates a handle for this counter. The labels should
// contain the keys and values for each key specified in the counter
// with the WithKeys option.
//
// If the labels do not contain a value for the key specified in the
// counter with the WithKeys option, then the missing value will be
// treated as unspecified.
func (c *Int64Counter) AcquireHandle(labels LabelSet) (h Int64CounterHandle) {
	h.commonHandle = c.acquireCommonHandle(labels)
	return
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c *Float64Counter) Measurement(value float64) Measurement {
	return c.float64Measurement(value)
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c *Int64Counter) Measurement(value int64) Measurement {
	return c.int64Measurement(value)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values for each key specified in the counter with the
// WithKeys option.
//
// If the labels do not contain a value for the key specified in the
// counter with the WithKeys option, then the missing value will be
// treated as unspecified.
func (c *Float64Counter) Add(ctx context.Context, value float64, labels LabelSet) {
	c.recordOne(ctx, core.NewFloat64Number(value), labels)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values for each key specified in the counter with the
// WithKeys option.
//
// If the labels do not contain a value for the key specified in the
// counter with the WithKeys option, then the missing value will be
// treated as unspecified.
func (c *Int64Counter) Add(ctx context.Context, value int64, labels LabelSet) {
	c.recordOne(ctx, core.NewInt64Number(value), labels)
}

// Add adds the value to the counter's sum.
func (h *Float64CounterHandle) Add(ctx context.Context, value float64) {
	h.recordOne(ctx, core.NewFloat64Number(value))
}

// Add adds the value to the counter's sum.
func (h *Int64CounterHandle) Add(ctx context.Context, value int64) {
	h.recordOne(ctx, core.NewInt64Number(value))
}
