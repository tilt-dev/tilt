package client

import "context"

type Alert struct {
	AlertType    string `json:"alertType"`
	ResourceName string `json:"resourceName"`
	Header       string `json:"header"`
	Msg          string `json:"msg"`
	RFC3339Time  string `json:"rfc3339Time"`
}

type AlertID string

type Client interface {
	SendAlert(ctx context.Context, alert Alert) (AlertID, error)
}

type tftClient struct{}

func (t *tftClient) SendAlert(ctx context.Context, alert Alert) (AlertID, error) {
	// TODO(dmiller): implement this
	return "", nil
}

func ProvideClient() Client {
	return &tftClient{}
}
