package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

type ManagerOpt func(manager *Manager)
type ReadDocumentFunc func(uri.URI) ([]byte, error)
type DocumentMap map[uri.URI]Document

// Manager provides simplified file read/write operations for the LSP server.
type Manager struct {
	mu          sync.Mutex
	root        uri.URI
	docs        DocumentMap
	newDocFunc  NewDocumentFunc
	readDocFunc ReadDocumentFunc
}

func NewDocumentManager(opts ...ManagerOpt) *Manager {
	m := Manager{
		docs:        make(DocumentMap),
		newDocFunc:  NewDocument,
		readDocFunc: ReadDocument,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return &m
}

func WithNewDocumentFunc(newDocFunc NewDocumentFunc) ManagerOpt {
	return func(manager *Manager) {
		manager.newDocFunc = newDocFunc
	}
}

func WithReadDocumentFunc(readDocFunc ReadDocumentFunc) ManagerOpt {
	return func(manager *Manager) {
		manager.readDocFunc = readDocFunc
	}
}

// Read the document from the given URI and return its contents. This default
// implementation of a ReadDocumentFunc only handles file: URIs and returns an
// error otherwise.
func ReadDocument(u uri.URI) (contents []byte, err error) {
	fn, err := filename(u)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fn)
}

func filename(u uri.URI) (fn string, err error) {
	defer func() {
		// recover from non-file URI in uri.Filename()
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	return u.Filename(), err
}

func canonicalFileURI(u uri.URI, base uri.URI) uri.URI {
	fn, err := filename(u)
	if err != nil {
		return u
	}
	if !filepath.IsAbs(fn) && base != "" {
		basepath, err := filename(base)
		if err != nil {
			return u
		}
		fn = filepath.Join(basepath, fn)
	}
	fn, err = filepath.EvalSymlinks(fn)
	if err != nil {
		return u
	}
	return uri.File(fn)
}

func (m *Manager) Initialize(params *protocol.InitializeParams) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(params.WorkspaceFolders) > 0 {
		m.root = uri.URI(params.WorkspaceFolders[0].URI)
	} else {
		dir, err := os.Getwd()
		if err == nil {
			m.root = uri.File(dir)
		}
	}
}

// Read returns the contents of the file for the given URI.
//
// If no file exists at the path or the URI is of an invalid type, an error is
// returned.
func (m *Manager) Read(ctx context.Context, u uri.URI) (doc Document, err error) {
	m.mu.Lock()
	defer func() {
		if err == nil {
			// Always return a copy of the document
			doc = doc.Copy()
		}
		m.mu.Unlock()
	}()
	u = canonicalFileURI(u, m.root)

	// TODO(siegs): check staleness for files read from disk?
	var found bool
	if doc, found = m.docs[u]; !found {
		doc, err = m.readAndParse(ctx, u, nil)
	}

	if os.IsNotExist(err) {
		err = os.ErrNotExist
	}

	return doc, err
}

// Write creates or replaces the contents of the file for the given URI.
func (m *Manager) Write(ctx context.Context, u uri.URI, input []byte) (diags []protocol.Diagnostic, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u = canonicalFileURI(u, m.root)
	m.removeAndCleanup(u)
	doc, err := m.parse(ctx, u, input, nil)
	if err != nil {
		return nil, fmt.Errorf("could not parse file %q: %v", u, err)
	}
	return doc.Diagnostics(), err
}

func (m *Manager) Remove(u uri.URI) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u = canonicalFileURI(u, m.root)
	m.removeAndCleanup(u)
}

func (m *Manager) Keys() []uri.URI {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]uri.URI, 0, len(m.docs))
	for k := range m.docs {
		keys = append(keys, k)
	}
	return keys
}

func (m *Manager) readAndParse(ctx context.Context, u uri.URI, parseState DocumentMap) (doc Document, err error) {
	var contents []byte
	u = canonicalFileURI(u, m.root)
	if _, found := m.docs[u]; !found {
		contents, err = m.readDocFunc(u)
		if err != nil {
			return nil, err
		}
	}
	return m.parse(ctx, u, contents, parseState)
}

func (m *Manager) parse(ctx context.Context, uri uri.URI, input []byte, parseState DocumentMap) (doc Document, err error) {
	cleanup := false
	if parseState == nil {
		parseState = make(DocumentMap)
		cleanup = true
	}

	if _, parsed := parseState[uri]; parsed {
		return nil, fmt.Errorf("circular load: %v", uri)
	}

	doc, loaded := m.docs[uri]
	if !loaded {
		tree, err := query.Parse(ctx, input)
		if err != nil {
			return nil, err
		}

		doc = m.newDocFunc(uri, input, tree)
	}

	parseState[uri] = doc
	if docx, ok := doc.(*document); ok {
		docx.followLoads(ctx, m, parseState)
	}

	if cleanup {
		for u, d := range parseState {
			m.docs[u] = d
		}
	}
	return doc, err
}

// removeAndCleanup removes a Document and frees associated resources.
func (m *Manager) removeAndCleanup(uri uri.URI) {
	if existing, ok := m.docs[uri]; ok {
		existing.Close()
	}
	delete(m.docs, uri)
}
