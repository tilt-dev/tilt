package query

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
)

func Parse(ctx context.Context, input []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(LanguagePython)

	tree, err := parser.ParseCtx(ctx, nil, input)
	if err != nil {
		return nil, err
	}

	return tree, nil
}
