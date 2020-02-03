package store

import (
	"fmt"
	"io"
	"unsafe"

	json "github.com/json-iterator/go"
	"github.com/json-iterator/go/extra"
	"github.com/modern-go/reflect2"

	"github.com/windmilleng/tilt/pkg/model"
)

var defaultJSONIterator = json.Config{}.Froze()

func CreateEngineStateEncoder(w io.Writer) *json.Encoder {
	config := json.Config{SortMapKeys: true}.Froze()
	config.RegisterExtension(&extra.BinaryAsStringExtension{})
	config.RegisterExtension(newEngineStateExtension())
	config.RegisterExtension(&privateFieldsExtension{})
	return config.NewEncoder(w)
}

type targetIDEncoder struct {
	delegate json.ValEncoder
}

func (targetIDEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	tID := (*model.TargetID)(ptr)
	return tID.Empty()
}

func (e targetIDEncoder) Encode(ptr unsafe.Pointer, stream *json.Stream) {
	tID := (*model.TargetID)(ptr)
	s := tID.String()
	stream.WriteString(fmt.Sprintf("%q", s))
}

type engineStateExtension struct {
	*json.DummyExtension
	targetIDType reflect2.Type
}

func newEngineStateExtension() engineStateExtension {
	return engineStateExtension{
		// memoize the type lookup
		targetIDType: reflect2.TypeOf(model.TargetID{}),
	}
}

func (e engineStateExtension) CreateMapKeyEncoder(typ reflect2.Type) json.ValEncoder {
	if e.targetIDType == typ {
		return targetIDEncoder{delegate: defaultJSONIterator.EncoderOf(typ)}
	}
	return nil
}

func (e engineStateExtension) CreateEncoder(typ reflect2.Type) json.ValEncoder {
	if e.targetIDType == typ {
		return targetIDEncoder{delegate: defaultJSONIterator.EncoderOf(typ)}
	}
	return nil
}
