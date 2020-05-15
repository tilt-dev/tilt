package temp

// An interface for creating and deleting directories.
type Dir interface {
	// Create a new directory under the current.
	//
	// The TempDir implementation will add a nonce to prevent problems
	// if a directory already exists with this name. The PersistentDir implementation
	// will just fail hard if a directory already exists.
	NewDir(name string) (Dir, error)

	Path() string

	TearDown() error
}
