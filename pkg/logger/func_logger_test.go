package logger

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFuncLogger_Level(t *testing.T) {
	out := &bytes.Buffer{}
	fl := NewFuncLogger(true, InfoLvl, func(level Level, fields Fields, b []byte) error {
		_, err := out.Write(b)
		return err
	})

	fl.Infof("info")
	fl.Debugf("debug")

	s := out.String()
	require.Contains(t, s, "info")
	require.NotContains(t, s, "debug")
}

func TestFuncLoggerWriter_Level(t *testing.T) {
	out := &bytes.Buffer{}
	fl := NewFuncLogger(true, InfoLvl, func(level Level, fields Fields, b []byte) error {
		_, err := out.Write(b)
		return err
	})

	_, _ = fl.Writer(InfoLvl).Write([]byte("info\n"))
	_, _ = fl.Writer(DebugLvl).Write([]byte("debug\n"))

	s := out.String()
	require.Contains(t, s, "info")
	require.NotContains(t, s, "debug")
}

func TestOneField(t *testing.T) {
	out := &bytes.Buffer{}
	fl := NewFuncLogger(true, InfoLvl, func(level Level, fields Fields, b []byte) error {
		_, err := fmt.Fprintf(out, "[%v]: %s", fields, string(b))
		return err
	})

	fl.WithFields(Fields{"a": "1"}).Infof("info")
	s := out.String()
	require.Equal(t, s, "[map[a:1]]: info\n")
}

func TestFieldLayering(t *testing.T) {
	out := &bytes.Buffer{}
	fl := NewFuncLogger(true, InfoLvl, func(level Level, fields Fields, b []byte) error {
		_, err := fmt.Fprintf(out, "[%v]: %s", fields, string(b))
		return err
	})

	l1 := fl.WithFields(Fields{"a": "1"})
	l1.WithFields(Fields{"b": "2"}).Infof("line1")
	l1.WithFields(Fields{"c": "3"}).Infof("line2")
	s := out.String()
	require.Equal(t, s, "[map[a:1 b:2]]: line1\n[map[a:1 c:3]]: line2\n")
}
