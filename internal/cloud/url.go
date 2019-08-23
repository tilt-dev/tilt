package cloud

import (
	"fmt"
	"os"

	"github.com/windmilleng/tilt/internal/token"
)

// an address like cloud.tilt.dev or localhost:10450
type Address string

const addressEnvName = "TILT_CLOUD_ADDRESS"

func ProvideAddress() Address {
	address := os.Getenv(addressEnvName)
	if address == "" {
		address = "alerts.tilt.dev"
	}

	return Address(address)
}

func RegisterTokenURL(cloudAddress string, t token.Token) string {
	return fmt.Sprintf("https://%s/register_token?token=%s", cloudAddress, t)
}
