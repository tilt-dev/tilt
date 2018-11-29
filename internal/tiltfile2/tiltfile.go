package tiltfile2

import ()

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
}

func Load(ctx context.Context, filename string) ([]model.Manifest, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) {
			logger.Get(ctx).Infof("%s", msg)
		},
	}

	s := newTiltfileState(ctx, filename)

	s.exec()
	s.analyze()
	return s.translate()
}

func skylarkStringDictToGoMap(d *skylark.Dict) (map[string]string, error) {
	r := map[string]string{}

	for _, tuple := range d.Items() {
		kV, ok := tuple[0].(skylark.String)
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
		}

		k := string(kV)

		vV, ok := tuple[1].(skylark.String)
		if !ok {
			return nil, fmt.Errorf("value is not a string: %T (%v)", tuple[1], tuple[1])
		}

		v := string(vV)

		r[k] = v
	}

	return r, nil
}
