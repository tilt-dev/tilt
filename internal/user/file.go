package user

import (
	"fmt"
	"os"

	"github.com/tilt-dev/wmclient/pkg/dirs"
	"gopkg.in/yaml.v3"
)

const userPrefsFileName = "tilt_user_prefs.yaml"

type filePrefs struct {
	dir *dirs.TiltDevDir
}

func NewFilePrefs(dir *dirs.TiltDevDir) *filePrefs {
	return &filePrefs{dir: dir}
}

func (f *filePrefs) Get() (Prefs, error) {
	contents, err := f.dir.ReadFile(userPrefsFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return Prefs{}, nil
		}
		return Prefs{}, fmt.Errorf("get user prefs: %v", err)
	}

	prefs := Prefs{}
	err = yaml.Unmarshal([]byte(contents), &prefs)
	if err != nil {
		return Prefs{}, fmt.Errorf("get user prefs: %v", err)
	}

	return prefs, nil
}

func (f *filePrefs) Update(newPrefs Prefs) error {
	contents, err := yaml.Marshal(newPrefs)
	if err != nil {
		return fmt.Errorf("update user prefs: %v", err)
	}
	err = f.dir.WriteFile(userPrefsFileName, string(contents))
	if err != nil {
		return fmt.Errorf("update user prefs: %v", err)
	}
	return nil
}
