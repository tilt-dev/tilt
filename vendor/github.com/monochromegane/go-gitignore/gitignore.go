package gitignore

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type IgnoreMatcher interface {
	Match(path string, isDir bool) bool
}

type gitIgnore struct {
	ignorePatterns scanStrategy
	acceptPatterns scanStrategy
	path           string
}

func NewGitIgnore(gitignore string, base ...string) (IgnoreMatcher, error) {
	var path string
	if len(base) > 0 {
		path = base[0]
	} else {
		path = filepath.Dir(gitignore)
	}

	file, err := os.Open(gitignore)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return NewGitIgnoreFromReader(path, file), nil
}

func NewGitIgnoreFromReader(path string, r io.Reader) gitIgnore {
	g := gitIgnore{
		ignorePatterns: newIndexScanPatterns(),
		acceptPatterns: newIndexScanPatterns(),
		path:           path,
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " ")
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "!") {
			g.acceptPatterns.add(strings.TrimPrefix(line, "!"))
		} else {
			g.ignorePatterns.add(line)
		}
	}
	return g
}

func (g gitIgnore) Match(path string, isDir bool) bool {
	relativePath, err := filepath.Rel(g.path, path)
	if err != nil {
		return false
	}

	if g.acceptPatterns.match(relativePath, isDir) {
		return false
	}
	return g.ignorePatterns.match(relativePath, isDir)
}
