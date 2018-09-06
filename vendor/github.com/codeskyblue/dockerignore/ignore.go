package dockerignore

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/scanner"
)

// exclusion return true if the specified pattern is an exclusion
func exclusion(pattern string) bool {
	return pattern[0] == '!'
}

// empty return true if the specified pattern is empty
func empty(pattern string) bool {
	return pattern == "" || strings.HasPrefix(pattern, "#")
}

// Read ignore from file
func ReadIgnoreFile(filename string) ([]string, error) {
	igrd, err := os.Open(filename)
	if err != nil {
		//if os.IsNotExist(err){
		//    return []string{}, nil
		//}
		return nil, err
	}
	return ReadIgnore(igrd)
}

// ReadIgnore reads a .dockerignore file and returns the list of file patterns
// to ignore. Note this will trim whitespace from each line as well
// as use GO's "clean" func to get the shortest/cleanest path for each.
func ReadIgnore(reader io.ReadCloser) ([]string, error) {
	if reader == nil {
		return nil, nil
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	var excludes []string

	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if empty(pattern) {
			continue
		}
		pattern = filepath.Clean(pattern)
		excludes = append(excludes, pattern)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading .dockerignore: %v", err)
	}
	return excludes, nil
}

// CleanPatterns takes a slice of patterns returns a new
// slice of patterns cleaned with filepath.Clean, stripped
// of any empty patterns and lets the caller know whether the
// slice contains any exception patterns (prefixed with !).
func cleanPatterns(patterns []string) ([]string, [][]string, bool, error) {
	// Loop over exclusion patterns and:
	// 1. Clean them up.
	// 2. Indicate whether we are dealing with any exception rules.
	// 3. Error if we see a single exclusion marker on it's own (!).
	cleanedPatterns := []string{}
	patternDirs := [][]string{}
	exceptions := false
	for _, pattern := range patterns {
		// Eliminate leading and trailing whitespace.
		pattern = strings.TrimSpace(pattern)
		if empty(pattern) {
			continue
		}
		if exclusion(pattern) {
			if len(pattern) == 1 {
				return nil, nil, false, errors.New("Illegal exclusion pattern: !")
			}
			exceptions = true
		}
		pattern = filepath.Clean(pattern)
		cleanedPatterns = append(cleanedPatterns, pattern)
		if exclusion(pattern) {
			pattern = pattern[1:]
		}
		patternDirs = append(patternDirs, strings.Split(pattern, "/"))
	}

	return cleanedPatterns, patternDirs, exceptions, nil
}

// Matches returns true if file matches any of the patterns
// and isn't excluded by any of the subsequent patterns.
func Matches(file string, patterns []string) (bool, error) {
	file = filepath.Clean(file)

	if file == "." {
		// Don't let them exclude everything, kind of silly.
		return false, nil
	}

	patterns, patDirs, _, err := cleanPatterns(patterns)
	if err != nil {
		return false, err
	}

	return optimizedMatches(file, patterns, patDirs)
}

// OptimizedMatches is basically the same as fileutils.Matches() but optimized for archive.go.
// It will assume that the inputs have been preprocessed and therefore the function
// doesn't need to do as much error checking and clean-up. This was done to avoid
// repeating these steps on each file being checked during the archive process.
// The more generic fileutils.Matches() can't make these assumptions.
func optimizedMatches(file string, patterns []string, patDirs [][]string) (bool, error) {
	matched := false
	parentPath := filepath.Dir(file)
	parentPathDirs := strings.Split(parentPath, "/")

	for i, pattern := range patterns {
		negative := false

		if exclusion(pattern) {
			negative = true
			pattern = pattern[1:]
		}

		match, err := regexpMatch(pattern, file)
		if err != nil {
			return false, fmt.Errorf("Error in pattern (%s): %s", pattern, err)
		}

		if !match && parentPath != "." {
			// Check to see if the pattern matches one of our parent dirs.
			if len(patDirs[i]) <= len(parentPathDirs) {
				match, _ = regexpMatch(strings.Join(patDirs[i], "/"),
					strings.Join(parentPathDirs[:len(patDirs[i])], "/"))
			}
		}

		if match {
			matched = !negative
		}
	}

	//if matched {
	//log.Debugf("Skipping excluded path: %s", file)
	//}

	return matched, nil
}

// regexpMatch tries to match the logic of filepath.Match but
// does so using regexp logic. We do this so that we can expand the
// wildcard set to include other things, like "**" to mean any number
// of directories.  This means that we should be backwards compatible
// with filepath.Match(). We'll end up supporting more stuff, due to
// the fact that we're using regexp, but that's ok - it does no harm.
func regexpMatch(pattern, path string) (bool, error) {
	regStr := "^"

	// Do some syntax checking on the pattern.
	// filepath's Match() has some really weird rules that are inconsistent
	// so instead of trying to dup their logic, just call Match() for its
	// error state and if there is an error in the pattern return it.
	// If this becomes an issue we can remove this since its really only
	// needed in the error (syntax) case - which isn't really critical.
	if _, err := filepath.Match(pattern, path); err != nil {
		return false, err
	}

	// Go through the pattern and convert it to a regexp.
	// We use a scanner so we can support utf-8 chars.
	var scan scanner.Scanner
	scan.Init(strings.NewReader(pattern))

	sl := string(os.PathSeparator)
	escSL := sl
	if sl == `\` {
		escSL += `\`
	}

	for scan.Peek() != scanner.EOF {
		ch := scan.Next()

		if ch == '*' {
			if scan.Peek() == '*' {
				// is some flavor of "**"
				scan.Next()

				if scan.Peek() == scanner.EOF {
					// is "**EOF" - to align with .gitignore just accept all
					regStr += ".*"
				} else {
					// is "**"
					regStr += "((.*" + escSL + ")|([^" + escSL + "]*))"
				}

				// Treat **/ as ** so eat the "/"
				if string(scan.Peek()) == sl {
					scan.Next()
				}
			} else {
				// is "*" so map it to anything but "/"
				regStr += "[^" + escSL + "]*"
			}
		} else if ch == '?' {
			// "?" is any char except "/"
			regStr += "[^" + escSL + "]"
		} else if strings.Index(".$", string(ch)) != -1 {
			// Escape some regexp special chars that have no meaning
			// in golang's filepath.Match
			regStr += `\` + string(ch)
		} else if ch == '\\' {
			// escape next char. Note that a trailing \ in the pattern
			// will be left alone (but need to escape it)
			if sl == `\` {
				// On windows map "\" to "\\", meaning an escaped backslash,
				// and then just continue because filepath.Match on
				// Windows doesn't allow escaping at all
				regStr += escSL
				continue
			}
			if scan.Peek() != scanner.EOF {
				regStr += `\` + string(scan.Next())
			} else {
				regStr += `\`
			}
		} else {
			regStr += string(ch)
		}
	}

	regStr += "$"

	res, err := regexp.MatchString(regStr, path)

	// Map regexp's error to filepath's so no one knows we're not using filepath
	if err != nil {
		err = filepath.ErrBadPattern
	}

	return res, err
}
