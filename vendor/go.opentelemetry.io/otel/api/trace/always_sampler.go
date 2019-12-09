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
	"go.opentelemetry.io/otel/api/core"
)

const (
	alwaysSamplerDescription = "AlwaysSampleSampler"
)

var alwaysSampleDecision = Decision{Sampled: true}

type alwaysSampleSampler struct{}

// ShouldSample implements Sampler interface.
// It always returns a Decision with Sampled value set to true
// and with Attributes set to an empty slice.
func (as alwaysSampleSampler) ShouldSample(
	_ core.SpanContext,
	_ bool,
	_ core.TraceID,
	_ uint64,
	_ string,
) Decision {
	return alwaysSampleDecision
}

// Description implements Sampler interface.
// It returns the description of this sampler.
func (as alwaysSampleSampler) Description() string {
	return alwaysSamplerDescription
}

var _ Sampler = alwaysSampleSampler{}

func AlwaysSampleSampler() Sampler {
	return alwaysSampleSampler{}
}
