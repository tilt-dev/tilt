package cloud

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/token"
)

//TODO TFT: change snapshot url to be cloud.tilt.dev
const Domain = "alerts.tilt.dev"

func RegisterTokenURL(t token.Token) string {
	return fmt.Sprintf("https://%s/register_token?token=%s", Domain, t)
}
