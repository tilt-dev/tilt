package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tonistiigi/fsutil"
	"github.com/tonistiigi/fsutil/types"
)

type ChangeKind = fsutil.ChangeKind

var ChangeKindAdd = fsutil.ChangeKindAdd
var ChangeKindModify = fsutil.ChangeKindModify
var ChangeKindDelete = fsutil.ChangeKindDelete

type StatInfo = fsutil.StatInfo

type change struct {
	kind ChangeKind
	path string
	fi   os.FileInfo
	data string
}

func changeStream(dt []string) (changes []*change) {
	for _, s := range dt {
		changes = append(changes, parseChange(s))
	}
	return
}

func parseChange(str string) *change {
	f := strings.Fields(str)
	errStr := fmt.Sprintf("invalid change %q", str)
	if len(f) < 3 {
		panic(errStr)
	}
	c := &change{}
	switch f[0] {
	case "ADD":
		c.kind = ChangeKindAdd
	case "CHG":
		c.kind = ChangeKindModify
	case "DEL":
		c.kind = ChangeKindDelete
	default:
		panic(errStr)
	}
	c.path = filepath.FromSlash(f[1])
	st := &types.Stat{}
	switch f[2] {
	case "file":
		if len(f) > 3 {
			if f[3][0] == '>' {
				st.Linkname = f[3][1:]
			} else {
				c.data = f[3]
			}
		}
	case "dir":
		st.Mode |= uint32(os.ModeDir)
	case "socket":
		st.Mode |= uint32(os.ModeSocket)
	case "symlink":
		if len(f) < 4 {
			panic(errStr)
		}
		st.Mode |= uint32(os.ModeSymlink)
		st.Linkname = f[3]
	}
	c.fi = &StatInfo{st}
	return c
}
