package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog_AppendUnderLimit(t *testing.T) {
	l := NewLog("foo")
	l = AppendLog(l, []byte("bar"))
	assert.Equal(t, "foobar", l.String())
}

func TestLog_AppendOverLimit(t *testing.T) {
	l := NewLog("hello\n")
	sb := strings.Builder{}
	for i := 0; i < maxLogLengthInBytes/2; i++ {
		_, err := sb.WriteString("x\n")
		if err != nil {
			t.Fatalf("error in %T.WriteString: %+v", sb, err)
		}
	}

	s := sb.String()

	l = AppendLog(l, []byte(s))

	assert.Equal(t, s, l.String())
}
