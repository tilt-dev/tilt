package dockerfile

import (
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

type Dockerfile string

func (d Dockerfile) Empty() bool {
	return d.String() == ""
}

// Find all images referenced in this dockerfile.
func (d Dockerfile) FindImages(buildArgs []string) ([]reference.Named, error) {
	result := []reference.Named{}
	ast, err := ParseAST(d)
	if err != nil {
		return nil, err
	}

	err = ast.traverseImageRefs(func(node *parser.Node, ref reference.Named) reference.Named {
		result = append(result, ref)
		return nil
	}, argInstructions(buildArgs))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (d Dockerfile) String() string {
	return string(d)
}
