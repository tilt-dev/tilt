package k8s

import (
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

var defaultJSONIterator = createDefaultJSONIterator()
var specJSONIterator = createSpecJSONIterator()

func createDefaultJSONConfig() jsoniter.Config {
	return jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		CaseSensitive:          true,
	}
}

func createDefaultJSONIterator() jsoniter.API {
	return createDefaultJSONConfig().Froze()
}

// Create a JSON iterator that:
// - encodes "zero" metav1.Time values as empty instead of nil
// - encodes all status values as empty
func createSpecJSONIterator() jsoniter.API {
	config := createDefaultJSONConfig().Froze()
	config.RegisterExtension(newTimeExtension())
	config.RegisterExtension(alwaysEmptyExtension{
		typeIndex: createTypeIndex(allStatusTypes()),
	})
	return config
}

func allStatusTypes() []reflect2.Type {
	result := []reflect2.Type{}
	for _, typ := range scheme.Scheme.AllKnownTypes() {
		typ2 := reflect2.Type2(typ)

		sTyp2, ok := typ2.(reflect2.StructType)
		if !ok {
			continue
		}

		statusField := sTyp2.FieldByName("Status")
		if statusField == nil {
			continue
		}

		result = append(result, statusField.Type())
	}
	return result
}

type TypeIndex map[reflect2.Type]bool

func (idx TypeIndex) Contains(typ reflect2.Type) bool {
	_, ok := idx[typ]
	return ok
}

func createTypeIndex(ts []reflect2.Type) TypeIndex {
	result := make(map[reflect2.Type]bool)
	for _, t := range ts {
		result[t] = true
	}
	return TypeIndex(result)
}

// Any type that matches this extension is considered empty,
// and skipped during json serialization.
type alwaysEmptyExtension struct {
	*jsoniter.DummyExtension
	typeIndex TypeIndex
}

func (e alwaysEmptyExtension) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	if e.typeIndex.Contains(typ) {
		return alwaysEmptyEncoder{}
	}
	return nil
}

type alwaysEmptyEncoder struct {
}

func (alwaysEmptyEncoder) IsEmpty(ptr unsafe.Pointer) bool                    { return true }
func (alwaysEmptyEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {}

type timeExtension struct {
	*jsoniter.DummyExtension
	timeType reflect2.Type
}

func newTimeExtension() timeExtension {
	return timeExtension{
		// memoize the type lookup
		timeType: reflect2.TypeOf(metav1.Time{}),
	}
}

func (e timeExtension) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	if e.timeType == typ {
		return timeEncoder{delegate: defaultJSONIterator.EncoderOf(typ)}
	}
	return nil
}

type timeEncoder struct {
	delegate jsoniter.ValEncoder
}

// Returns true if the time value is the zero value.
func (e timeEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	t := *((*metav1.Time)(ptr))
	return t == metav1.Time{} || e.delegate.IsEmpty(ptr)
}

func (e timeEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	e.delegate.Encode(ptr, stream)
}
