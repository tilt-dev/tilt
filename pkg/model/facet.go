package model

import "github.com/windmilleng/tilt/pkg/webview"

type Facet struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	SpanID string `json:"span_id"`
}

func (f Facet) ToProto() *webview.Facet {
	return &webview.Facet{
		Name:   f.Name,
		Value:  f.Value,
		SpanId: f.SpanID,
	}
}

func FacetsToProto(facets []Facet) []*webview.Facet {
	ret := make([]*webview.Facet, len(facets))
	for i, f := range facets {
		ret[i] = f.ToProto()
	}

	return ret
}
