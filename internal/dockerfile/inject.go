package dockerfile

import (
	"github.com/docker/distribution/reference"
)

func InjectImageDigest(df Dockerfile, ref reference.NamedTagged) (Dockerfile, bool, error) {
	ast, err := ParseAST(df)
	if err != nil {
		return "", false, err
	}

	modified, err := ast.InjectImageDigest(ref)
	if err != nil {
		return "", false, err
	}

	if !modified {
		return df, false, nil
	}

	newDf, err := ast.Print()
	return newDf, true, err
}
