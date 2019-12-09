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
	"encoding/binary"

	"go.opentelemetry.io/otel/api/core"
)

const defaultSamplingProbability = 1e-4

// Sampler decides whether a trace should be sampled and exported.
type Sampler func(SamplingParameters) SamplingDecision

// SamplingParameters contains the values passed to a Sampler.
type SamplingParameters struct {
	ParentContext   core.SpanContext
	TraceID         core.TraceID
	SpanID          core.SpanID
	Name            string
	HasRemoteParent bool
}

// SamplingDecision is the value returned by a Sampler.
type SamplingDecision struct {
	Sample bool
}

// ProbabilitySampler samples a given fraction of traces. Fractions >= 1 will
// always sample. If the parent span is sampled, then it's child spans will
// automatically be sampled. Fractions <0 are treated as zero, but spans may
// still be sampled if their parent is.
func ProbabilitySampler(fraction float64) Sampler {
	if fraction >= 1 {
		return AlwaysSample()
	}

	if fraction <= 0 {
		fraction = 0
	}
	traceIDUpperBound := uint64(fraction * (1 << 63))
	return Sampler(func(p SamplingParameters) SamplingDecision {
		if p.ParentContext.IsSampled() {
			return SamplingDecision{Sample: true}
		}
		x := binary.BigEndian.Uint64(p.TraceID[0:8]) >> 1
		return SamplingDecision{Sample: x < traceIDUpperBound}
	})
}

// AlwaysSample returns a Sampler that samples every trace.
// Be careful about using this sampler in a production application with
// significant traffic: a new trace will be started and exported for every
// request.
func AlwaysSample() Sampler {
	return func(p SamplingParameters) SamplingDecision {
		return SamplingDecision{Sample: true}
	}
}

// NeverSample returns a Sampler that samples no traces.
func NeverSample() Sampler {
	return func(p SamplingParameters) SamplingDecision {
		return SamplingDecision{Sample: false}
	}
}
