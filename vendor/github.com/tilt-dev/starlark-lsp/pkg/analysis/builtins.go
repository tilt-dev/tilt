package analysis

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/pkg/errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
var StarlarkBuiltins []byte

func NewBuiltins() *Builtins {
	return &Builtins{
		Functions: make(map[string]protocol.SignatureInformation),
		Symbols:   []protocol.DocumentSymbol{},
	}
}

func (b *Builtins) IsEmpty() bool {
	return len(b.Functions) == 0 && len(b.Symbols) == 0
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
		for _, path := range paths {
			builtins, err := LoadBuiltins(analyzer.context, os.DirFS(path))
			if err != nil {
				return err
			}
			analyzer.builtins.Update(builtins)
		}
		return nil
	}
}

func WithBuiltins(builtins fs.FS) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		builtins, err := LoadBuiltins(analyzer.context, builtins)
		if err != nil {
			return err
		}
		analyzer.builtins.Update(builtins)
		return nil
	}
}

func WithStarlarkBuiltins() AnalyzerOption {
	return func(analyzer *Analyzer) error {
		builtins, err := LoadBuiltinsFromSource(analyzer.context, StarlarkBuiltins, "builtins.py")
		if err != nil {
			return errors.Wrapf(err, "loading builtins from builtins.py")
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

func LoadBuiltinsFromSource(ctx context.Context, contents []byte, path string) (*Builtins, error) {
	tree, err := query.Parse(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q: %v", path, err)
	}

	functions := make(map[string]protocol.SignatureInformation)
	doc := document.NewDocument(contents, tree)
	docFunctions := query.Functions(doc, tree.RootNode())
	symbols := query.DocumentSymbols(doc)
	doc.Close()

	for fn, sig := range docFunctions {
		if _, ok := functions[fn]; ok {
			return nil, fmt.Errorf("duplicate function %q found in %q", fn, path)
		}
		functions[fn] = sig
	}

	return &Builtins{
		Functions: functions,
		Symbols:   symbols,
	}, nil
}

func LoadBuiltinsFromFile(ctx context.Context, path string, f fs.FS) (*Builtins, error) {
	var contents []byte
	var err error
	if f != nil {
		contents, err = fs.ReadFile(f, path)
	} else {
		contents, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", path)
	}
	return LoadBuiltinsFromSource(ctx, contents, path)
}

func loadBuiltinModuleWalker(ctx context.Context, f fs.FS) (map[string]*Builtins, fs.WalkDirFunc) {
	builtins := make(map[string]*Builtins)
	return builtins, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		entryName := entry.Name()

		if !entry.IsDir() && !strings.HasSuffix(entryName, ".py") {
			return nil
		}

		modPath := path

		if entry.IsDir() {
			builtins[modPath] = NewBuiltins()
			return nil
		}

		if entryName == "__init__.py" {
			modPath = filepath.Dir(modPath)
		} else {
			modPath = path[:len(path)-len(".py")]
		}

		modBuiltins, err := LoadBuiltinsFromFile(ctx, path, f)
		if err != nil {
			return errors.Wrapf(err, "loading builtins from %s", path)
		}

		if b, ok := builtins[modPath]; ok {
			b.Update(modBuiltins)
		} else {
			builtins[modPath] = modBuiltins
		}
		return nil
	}
}

func LoadBuiltinModuleFS(ctx context.Context, f fs.FS, root string) (*Builtins, error) {
	if root == "" {
		root = "."
	}

	builtinsMap, walker := loadBuiltinModuleWalker(ctx, f)
	err := fs.WalkDir(f, root, walker)

	if err != nil {
		return nil, errors.Wrapf(err, "walking %s", root)
	}

	modulePaths := make([]string, len(builtinsMap))
	i := 0
	for modPath := range builtinsMap {
		modulePaths[i] = modPath
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(modulePaths)))

	for _, modPath := range modulePaths {
		mod := builtinsMap[modPath]
		if mod.IsEmpty() || modPath == root {
			continue
		}

		modName := filepath.Base(modPath)
		parentModPath := filepath.Dir(modPath)
		parentMod, ok := builtinsMap[parentModPath]
		if !ok {
			return nil, fmt.Errorf("no entry for parent %s", parentModPath)
		}

		copyBuiltinsToParent(mod, parentMod, modName)
	}

	builtins, ok := builtinsMap[root]
	if !ok {
		return nil, fmt.Errorf("no entry for root %s", root)
	}
	return builtins, nil
}

func copyBuiltinsToParent(mod, parentMod *Builtins, modName string) {
	for name, fn := range mod.Functions {
		parentMod.Functions[modName+"."+name] = fn
	}

	children := []protocol.DocumentSymbol{}
	for _, sym := range mod.Symbols {
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
		existingIndex := -1
		for i, sym := range parentMod.Symbols {
			if sym.Name == modName {
				existingIndex = i
				break
			}
		}

		if existingIndex >= 0 {
			parentMod.Symbols[existingIndex].Children = append(parentMod.Symbols[existingIndex].Children, children...)
		} else {
			parentMod.Symbols = append(parentMod.Symbols, protocol.DocumentSymbol{
				Name:     modName,
				Kind:     protocol.SymbolKindVariable,
				Children: children,
			})
		}
	}
}

func LoadBuiltinModule(ctx context.Context, path string, fsys fs.FS) (*Builtins, error) {
	return LoadBuiltinModuleFS(ctx, fsys, "")
}

func LoadBuiltins(ctx context.Context, fsys fs.FS) (*Builtins, error) {
	builtins := NewBuiltins()

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fileInfo, err := fs.Stat(fsys, path)
		if err != nil {
			return errors.Wrapf(err, "statting %s", path)
		}
		var result *Builtins
		if fileInfo.IsDir() {
			result, err = LoadBuiltinModule(ctx, path, fsys)
		} else {
			result, err = LoadBuiltinsFromFile(ctx, path, fsys)
		}
		if err != nil {
			return errors.Wrapf(err, "loading builtins from %s", path)
		}
		builtins.Update(result)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return builtins, nil
}
