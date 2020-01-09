package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFuncLogger_Level(t *testing.T) {
	out := &bytes.Buffer{}
	fl := NewFuncLogger(true, InfoLvl, func(level Level, b []byte) error {
		_, err := out.Write(b)
		return err
	})

	fl.Infof("info")
	fl.Debugf("debug")

	s := out.String()
	require.Contains(t, s, "info")
	require.NotContains(t, s, "debug")
}
