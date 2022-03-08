package analysis

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

type Builtins struct {
	Functions map[string]protocol.SignatureInformation `json:"functions"`
	Symbols   []protocol.DocumentSymbol                `json:"symbols"`
}

//go:embed builtins.py
var starlarkBuiltins []byte

func NewBuiltins() *Builtins {
	return &Builtins{
		Functions: make(map[string]protocol.SignatureInformation),
		Symbols:   []protocol.DocumentSymbol{},
	}
}

func (b *Builtins) Update(other *Builtins) {
	if len(other.Functions) > 0 {
		for name, sig := range other.Functions {
			b.Functions[name] = sig
		}
	}
	if len(other.Symbols) > 0 {
		b.Symbols = append(b.Symbols, other.Symbols...)
	}
}

func (b *Builtins) FunctionNames() []string {
	names := make([]string, len(b.Functions))
	i := 0
	for name := range b.Functions {
		names[i] = name
		i++
	}
	return names
}

func (b *Builtins) SymbolNames() []string {
	names := make([]string, len(b.Symbols))
	for i, sym := range b.Symbols {
		names[i] = sym.Name
	}
	return names
}

func WithBuiltinPaths(paths []string) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		builtins, err := LoadBuiltins(analyzer.context, paths)
		if err != nil {
			return err
		}
		analyzer.builtins.Update(builtins)
		return nil
	}
}

func WithBuiltinFunctions(sigs map[string]protocol.SignatureInformation) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		analyzer.builtins.Update(&Builtins{Functions: sigs})
		return nil
	}
}

func WithBuiltinSymbols(symbols []protocol.DocumentSymbol) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		analyzer.builtins.Update(&Builtins{Symbols: symbols})
		return nil
	}
}

func WithStarlarkBuiltins() AnalyzerOption {
	return func(analyzer *Analyzer) error {
		builtins, err := LoadBuiltinsFromSource(analyzer.context, starlarkBuiltins, "builtins.py")
		if err != nil {
			return err
		}
		analyzer.builtins.Update(&Builtins{
			Symbols: []protocol.DocumentSymbol{
				{Name: "False", Kind: protocol.SymbolKindBoolean},
				{Name: "None", Kind: protocol.SymbolKindNull},
				{Name: "True", Kind: protocol.SymbolKindBoolean},
			},
		})
		analyzer.builtins.Update(builtins)
		return nil
	}
}

func LoadBuiltinsFromFile(ctx context.Context, path string) (*Builtins, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return &Builtins{}, err
	}
	return LoadBuiltinsFromSource(ctx, contents, path)
}

func LoadBuiltinsFromSource(ctx context.Context, contents []byte, path string) (*Builtins, error) {
	tree, err := query.Parse(ctx, contents)
	if err != nil {
		return &Builtins{}, fmt.Errorf("failed to parse %q: %v", path, err)
	}

	functions := make(map[string]protocol.SignatureInformation)
	doc := document.NewDocument(contents, tree)
	docFunctions := query.Functions(doc, tree.RootNode())
	symbols := query.DocumentSymbols(doc)
	doc.Close()

	for fn, sig := range docFunctions {
		if _, ok := functions[fn]; ok {
			return &Builtins{}, fmt.Errorf("duplicate function %q found in %q", fn, path)
		}
		functions[fn] = sig
	}

	return &Builtins{
		Functions: functions,
		Symbols:   symbols,
	}, nil
}

func LoadBuiltinModule(ctx context.Context, dir string) (*Builtins, error) {
	builtins := NewBuiltins()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		entryName := entry.Name()

		if !entry.IsDir() && !strings.HasSuffix(entryName, ".py") {
			continue
		}

		if entryName == "__init__.py" {
			initBuiltins, err := LoadBuiltinsFromFile(ctx, filepath.Join(dir, entryName))
			if err != nil {
				return nil, err
			}
			builtins.Update(initBuiltins)
			continue
		}

		var modName string
		var modBuiltins *Builtins
		if entry.IsDir() {
			modName = entryName
			modBuiltins, err = LoadBuiltinModule(ctx, filepath.Join(dir, entryName))
		} else {
			modName = entryName[:len(entryName)-len(".py")]
			modBuiltins, err = LoadBuiltinsFromFile(ctx, filepath.Join(dir, entryName))
		}

		if err != nil {
			return nil, err
		}

		for name, fn := range modBuiltins.Functions {
			builtins.Functions[modName+"."+name] = fn
		}

		children := []protocol.DocumentSymbol{}
		for _, sym := range modBuiltins.Symbols {
			var kind protocol.SymbolKind
			switch sym.Kind {
			case protocol.SymbolKindFunction:
				kind = protocol.SymbolKindMethod
			default:
				kind = protocol.SymbolKindField
			}
			childSym := sym
			childSym.Kind = kind
			children = append(children, childSym)
		}
		if len(children) > 0 {
			builtins.Symbols = append(builtins.Symbols, protocol.DocumentSymbol{
				Name:     modName,
				Kind:     protocol.SymbolKindVariable,
				Children: children,
			})
		}
	}
	return builtins, nil
}

func LoadBuiltins(ctx context.Context, filePaths []string) (*Builtins, error) {
	builtins := NewBuiltins()

	for _, path := range filePaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return &Builtins{}, err
		}
		var result *Builtins
		if fileInfo.IsDir() {
			result, err = LoadBuiltinModule(ctx, path)
		} else {
			result, err = LoadBuiltinsFromFile(ctx, path)
		}
		if err != nil {
			return &Builtins{}, err
		}
		builtins.Update(result)
	}

	return builtins, nil
}
