package encoding

import "github.com/windmilleng/tilt/internal/tiltfile/starkit"

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (Extension) OnStart(env *starkit.Environment) error {
	for _, b := range []struct {
		name string
		f    starkit.Function
	}{
		{"read_yaml", readYAML},
		{"decode_yaml", decodeYAML},

		{"read_json", readJSON},
		{"decode_json", decodeJSON},
	} {
		err := env.AddBuiltin(b.name, b.f)
		if err != nil {
			return err
		}
	}

	return nil
}
