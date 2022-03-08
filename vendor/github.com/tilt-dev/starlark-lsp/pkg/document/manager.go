package document

import (
	"os"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/uri"
)

type ManagerOpt func(manager *Manager)

// Manager provides simplified file read/write operations for the LSP server.
type Manager struct {
	mu         sync.Mutex
	docs       map[uri.URI]Document
	newDocFunc NewDocumentFunc
}

func NewDocumentManager(opts ...ManagerOpt) *Manager {
	m := Manager{
		docs:       make(map[uri.URI]Document),
		newDocFunc: NewDocument,
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

// Read returns the contents of the file for the given URI.
//
// If no file exists at the path or the URI is of an invalid type, an error is
// returned.
func (m *Manager) Read(uri uri.URI) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc, ok := m.docs[uri]; ok {
		return doc.Copy(), nil
	}
	return nil, os.ErrNotExist
}

// Write creates or replaces the contents of the file for the given URI.
func (m *Manager) Write(uri uri.URI, input []byte, tree *sitter.Tree) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeAndCleanup(uri)
	m.docs[uri] = m.newDocFunc(input, tree)
}

func (m *Manager) Remove(uri uri.URI) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeAndCleanup(uri)
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

// removeAndCleanup removes a Document and frees associated resources.
func (m *Manager) removeAndCleanup(uri uri.URI) {
	if existing, ok := m.docs[uri]; ok {
		existing.Close()
	}
	delete(m.docs, uri)
}
