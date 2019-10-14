package cloud

import (
	"os"
)

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
