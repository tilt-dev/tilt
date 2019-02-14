package dockerfile

import (
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/command"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

func InjectImageDigest(df Dockerfile, ref reference.NamedTagged) (Dockerfile, bool, error) {
	ast, err := ParseAST(df)
	if err != nil {
		return "", false, err
	}

	modified := false
	err = ast.Traverse(func(node *parser.Node) error {
		if node.Value != command.From || node.Next == nil {
			return nil
		}

		val := node.Next.Value
		fromRef, err := reference.ParseNormalizedNamed(val)
		if err != nil {
			// ignore the error
			return nil
		}

		if fromRef.Name() == ref.Name() {
			node.Next.Value = ref.String()
			modified = true
		}

		return nil
	})

	if err != nil {
		return "", false, err
	}

	if !modified {
		return df, false, nil
	}

	newDf, err := ast.Print()
	return newDf, true, err
}
