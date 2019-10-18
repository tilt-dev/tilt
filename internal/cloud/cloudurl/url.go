package cloudurl

import (
	"net/url"
	"os"
	"strings"
)

// this is in its own package to avoid circular dependencies

// an address like cloud.tilt.dev or localhost:10450
type Address string

const addressEnvName = "TILT_CLOUD_ADDRESS"

func ProvideAddress() Address {
	address := os.Getenv(addressEnvName)
	if address == "" {
		address = "cloud.tilt.dev"
	}

	return Address(address)
}

func URL(cloudAddress string) *url.URL {
	var u url.URL
	u.Host = cloudAddress
	u.Scheme = "https"
	if strings.Split(cloudAddress, ":")[0] == "localhost" {
		u.Scheme = "http"
	}
	return &u
}
