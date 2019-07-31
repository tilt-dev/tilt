package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

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

const alertStorageBaseURL = "https://alerts.tilt.dev"

type newAlertResponse struct {
	ID string
}

type tftClient struct{}

func (t *tftClient) SendAlert(ctx context.Context, alert Alert) (AlertID, error) {
	buf, err := json.Marshal(alert)
	if err != nil {
		return "", err
	}
	resp, err := http.Post(fmt.Sprintf("%s/api/alert", alertStorageBaseURL), "application/json", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Expected 200 response code from backend, got %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var r newAlertResponse
	err = json.Unmarshal(body, &r)
	return AlertID(r.ID), nil
}

func ProvideClient() Client {
	return &tftClient{}
}
