package analysis

import (
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

type Builtins struct {
	Functions map[string]query.Signature
	Symbols   []query.Symbol
	Types     map[string]query.Type
	Methods   map[string]query.Signature
	Members   []query.Symbol
}

//go:embed builtins.py
var StarlarkBuiltins []byte

func mapKeys(m interface{}) []string {
	val := reflect.ValueOf(m)
	keys := val.MapKeys()
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.String()
	}
	return names
}

func symbolNames(ss []query.Symbol) []string {
	names := make([]string, len(ss))
	for i, sym := range ss {
		names[i] = sym.Name
	}
	return names
}

func NewBuiltins() *Builtins {
	return &Builtins{
		Functions: make(map[string]query.Signature),
		Symbols:   []query.Symbol{},
		Types:     make(map[string]query.Type),
		Methods:   make(map[string]query.Signature),
		Members:   []query.Symbol{},
	}
}

func (b *Builtins) IsEmpty() bool {
	return len(b.Functions) == 0 && len(b.Symbols) == 0 &&
		len(b.Methods) == 0 && len(b.Members) == 0
}

func (b *Builtins) Update(other *Builtins) {
	if len(other.Functions) > 0 {
		for name, fn := range other.Functions {
			b.Functions[name] = fn
		}
	}
	if len(other.Symbols) > 0 {
		b.Symbols = append(b.Symbols, other.Symbols...)
	}
	if len(other.Types) > 0 {
		for name, t := range other.Types {
			b.Types[name] = t
		}
	}
	if len(other.Methods) > 0 {
		for name, fn := range other.Methods {
			b.Methods[name] = fn
		}
	}
	if len(other.Members) > 0 {
		b.Members = append(b.Members, other.Members...)
	}
}

func WithBuiltinPaths(paths []string) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		for _, path := range paths {
			builtins, err := LoadBuiltins(analyzer.context, path)
			if err != nil {
				return err
			}
			analyzer.builtins.Update(builtins)
		}
		return nil
	}
}

func WithBuiltins(f fs.FS) AnalyzerOption {
	return func(analyzer *Analyzer) error {
		builtins, err := LoadBuiltinsFromFS(analyzer.context, f)
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
			Symbols: []query.Symbol{
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
		return nil, errors.Wrapf(err, "failed to parse %q", path)
	}

	doc := document.NewDocument(uri.File(path), contents, tree)
	functions := doc.Functions()
	symbols := doc.Symbols()

	types := query.Types(doc, tree.RootNode())
	typeMap := make(map[string]query.Type)
	methodMap := make(map[string]query.Signature)
	members := []query.Symbol{}
	for _, t := range types {
		typeMap[t.Name] = t
		for _, method := range t.Methods {
			methodMap[method.Name] = method
		}
		members = append(members, t.Members...)
	}

	doc.Close()

	// NewDocument returns these symbols with a location of __init__.py, which isn't helpful to anyone
	for i, s := range symbols {
		s.Location = protocol.Location{}
		symbols[i] = s
	}

	return &Builtins{
		Functions: functions,
		Symbols:   symbols,
		Types:     typeMap,
		Methods:   methodMap,
		Members:   members,
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

func loadBuiltinsWalker(ctx context.Context, f fs.FS) (map[string]*Builtins, fs.WalkDirFunc) {
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

func LoadBuiltinsFromFS(ctx context.Context, f fs.FS) (*Builtins, error) {
	root := "."

	builtinsMap, walker := loadBuiltinsWalker(ctx, f)
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

	children := []query.Symbol{}
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
			parentMod.Symbols = append(parentMod.Symbols, query.Symbol{
				Name:     modName,
				Kind:     protocol.SymbolKindVariable,
				Children: children,
			})
		}
	}
	parentMod.Update(&Builtins{
		Types:   mod.Types,
		Methods: mod.Methods,
		Members: mod.Members,
	})
}

func LoadBuiltins(ctx context.Context, path string) (*Builtins, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "statting %s", path)
	}

	var result *Builtins
	if fileInfo.IsDir() {
		result, err = LoadBuiltinsFromFS(ctx, os.DirFS(path))
	} else {
		result, err = LoadBuiltinsFromFile(ctx, path, nil)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "loading builtins from %s", path)
	}

	return result, nil
}
