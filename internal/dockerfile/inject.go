package dockerfile

import (
	"github.com/distribution/reference"

	"github.com/tilt-dev/tilt/internal/container"
)

func InjectImageDigest(df Dockerfile, selector container.RefSelector, ref reference.NamedTagged, buildArgs []string) (Dockerfile, bool, error) {
	ast, err := ParseAST(df)
	if err != nil {
		return "", false, err
	}

	modified, err := ast.InjectImageDigest(selector, ref, buildArgs)
	if err != nil {
		return "", false, err
	}

	if !modified {
		return df, false, nil
	}

	newDf, err := ast.Print()
	return newDf, true, err
}
