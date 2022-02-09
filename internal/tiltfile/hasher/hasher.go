package hasher

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

type Hashes struct {
	TiltfileSHA256 string
	AllFilesSHA256 string
}

type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (e Plugin) NewState() interface{} {
	return hashState{tiltfile: sha256.New(), allFiles: sha256.New()}
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	return nil
}

func (e Plugin) OnExec(t *starlark.Thread, tiltfilePath string, contents []byte) error {
	return starkit.SetState(t, func(hs hashState) hashState {
		if hs.filesRead == 0 {
			hs.tiltfile.Write(contents)
		}
		hs.allFiles.Write(contents)
		hs.filesRead += 1
		return hs
	})
}

var _ starkit.StatefulPlugin = Plugin{}
var _ starkit.OnExecPlugin = Plugin{}

func MustState(model starkit.Model) hashState {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (hashState, error) {
	var state hashState
	err := m.Load(&state)
	return state, err
}

type hashState struct {
	filesRead int
	tiltfile  hash.Hash
	allFiles  hash.Hash
}

func (hs hashState) GetHashes() Hashes {
	if hs.filesRead == 0 || hs.tiltfile == nil {
		return Hashes{}
	}
	return Hashes{
		TiltfileSHA256: fmt.Sprintf("%x", hs.tiltfile.Sum(nil)),
		AllFilesSHA256: fmt.Sprintf("%x", hs.allFiles.Sum(nil)),
	}
}
