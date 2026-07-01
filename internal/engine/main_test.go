package engine

import (
	"io"
	"os"
	"testing"

	"github.com/tilt-dev/tilt/internal/controllers"
)

func TestMain(m *testing.M) {
	controllers.InitKlog(io.Discard)
	os.Exit(m.Run())
}
