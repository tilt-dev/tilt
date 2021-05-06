package v1

import (
	"encoding/json"

	"github.com/golang/protobuf/jsonpb"
)

// Marshal implements the protobuf marshaling interface.
func (m *Time) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {
	return json.Marshal(m)
}

func (m *MicroTime) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {
	return json.Marshal(m)
}

func (m *Time) UnmarshalJSONPB(marshaler *jsonpb.Unmarshaler, b []byte) error {
	return json.Unmarshal(b, m)
}

func (m *MicroTime) UnmarshalJSONPB(marshaler *jsonpb.Unmarshaler, b []byte) error {
	return json.Unmarshal(b, m)
}
