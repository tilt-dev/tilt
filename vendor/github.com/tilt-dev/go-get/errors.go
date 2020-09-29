package get

import (
	"errors"
	"fmt"
	"strings"
)

// ImportPathError is a type of error that prevents a package from being loaded
// for a given import path. When such a package is loaded, a *Package is
// returned with Err wrapping an ImportPathError: the error is attached to
// the imported package, not the importing package.
//
// The string returned by ImportPath must appear in the string returned by
// Error. Errors that wrap ImportPathError (such as PackageError) may omit
// the import path.
type ImportPathError interface {
	error
	ImportPath() string
}

type importError struct {
	importPath string
	err        error // created with fmt.Errorf
}

var _ ImportPathError = (*importError)(nil)

func ImportErrorf(path, format string, args ...interface{}) ImportPathError {
	err := &importError{importPath: path, err: fmt.Errorf(format, args...)}
	if errStr := err.Error(); !strings.Contains(errStr, path) {
		panic(fmt.Sprintf("path %q not in error %q", path, errStr))
	}
	return err
}

func (e *importError) Error() string {
	return e.err.Error()
}

func (e *importError) Unwrap() error {
	// Don't return e.err directly, since we're only wrapping an error if %w
	// was passed to ImportErrorf.
	return errors.Unwrap(e.err)
}

func (e *importError) ImportPath() string {
	return e.importPath
}
