// Package dotenv is a go port of the ruby dotenv library (https://github.com/bkeepers/dotenv)
//
// Examples/readme can be found on the github page at https://github.com/joho/godotenv
//
// The TL;DR is that you make a .env file that looks something like
//
//	SOME_ENV_VAR=somevalue
//
// and then in your go code you can call
//
//	godotenv.Load()
//
// and all the env vars declared in .env will be available through os.Getenv("SOME_ENV_VAR")
package dotenv

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/template"
)

var utf8BOM = []byte("\uFEFF")

var startsWithDigitRegex = regexp.MustCompile(`^\s*\d.*`) // Keys starting with numbers are ignored

// LookupFn represents a lookup function to resolve variables from
type LookupFn func(string) (string, bool)

var noLookupFn = func(s string) (string, bool) {
	return "", false
}

// Parse reads an env file from io.Reader, returning a map of keys and values.
func Parse(r io.Reader) (map[string]string, error) {
	return ParseWithLookup(r, nil)
}

// ParseWithLookup reads an env file from io.Reader, returning a map of keys and values.
func ParseWithLookup(r io.Reader, lookupFn LookupFn) (map[string]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// seek past the UTF-8 BOM if it exists (particularly on Windows, some
	// editors tend to add it, and it'll cause parsing to fail)
	data = bytes.TrimPrefix(data, utf8BOM)

	return UnmarshalBytesWithLookup(data, lookupFn)
}

// Load will read your env file(s) and load them into ENV for this process.
//
// Call this function as close as possible to the start of your program (ideally in main).
//
// If you call Load without any args it will default to loading .env in the current path.
//
// You can otherwise tell it which files to load (there can be more than one) like:
//
//	godotenv.Load("fileone", "filetwo")
//
// It's important to note that it WILL NOT OVERRIDE an env variable that already exists - consider the .env file to set dev vars or sensible defaults
func Load(filenames ...string) error {
	return load(false, filenames...)
}

// Overload will read your env file(s) and load them into ENV for this process.
//
// Call this function as close as possible to the start of your program (ideally in main).
//
// If you call Overload without any args it will default to loading .env in the current path.
//
// You can otherwise tell it which files to load (there can be more than one) like:
//
//	godotenv.Overload("fileone", "filetwo")
//
// It's important to note this WILL OVERRIDE an env variable that already exists - consider the .env file to forcefilly set all vars.
func Overload(filenames ...string) error {
	return load(true, filenames...)
}

func load(overload bool, filenames ...string) error {
	filenames = filenamesOrDefault(filenames)
	for _, filename := range filenames {
		err := loadFile(filename, overload)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadWithLookup gets all env vars from the files and/or lookup function and return values as
// a map rather than automatically writing values into env
func ReadWithLookup(lookupFn LookupFn, filenames ...string) (map[string]string, error) {
	filenames = filenamesOrDefault(filenames)
	envMap := make(map[string]string)

	for _, filename := range filenames {
		individualEnvMap, individualErr := readFile(filename, lookupFn)

		if individualErr != nil {
			return envMap, individualErr
		}

		for key, value := range individualEnvMap {
			if startsWithDigitRegex.MatchString(key) {
				continue
			}
			envMap[key] = value
		}
	}

	return envMap, nil
}

// Read all env (with same file loading semantics as Load) but return values as
// a map rather than automatically writing values into env
func Read(filenames ...string) (map[string]string, error) {
	return ReadWithLookup(nil, filenames...)
}

// Unmarshal reads an env file from a string, returning a map of keys and values.
func Unmarshal(str string) (map[string]string, error) {
	return UnmarshalBytes([]byte(str))
}

// UnmarshalBytes parses env file from byte slice of chars, returning a map of keys and values.
func UnmarshalBytes(src []byte) (map[string]string, error) {
	return UnmarshalBytesWithLookup(src, nil)
}

// UnmarshalBytesWithLookup parses env file from byte slice of chars, returning a map of keys and values.
func UnmarshalBytesWithLookup(src []byte, lookupFn LookupFn) (map[string]string, error) {
	out := make(map[string]string)
	err := parseBytes(src, out, lookupFn)
	return out, err
}

// Exec loads env vars from the specified filenames (empty map falls back to default)
// then executes the cmd specified.
//
// Simply hooks up os.Stdin/err/out to the command and calls Run()
//
// If you want more fine grained control over your command it's recommended
// that you use `Load()` or `Read()` and the `os/exec` package yourself.
//
// Deprecated: Use the `os/exec` package directly.
func Exec(filenames []string, cmd string, cmdArgs []string) error {
	if err := Load(filenames...); err != nil {
		return err
	}

	command := exec.Command(cmd, cmdArgs...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// Write serializes the given environment and writes it to a file
//
// Deprecated: The serialization functions are untested and unmaintained.
func Write(envMap map[string]string, filename string) error {
	//goland:noinspection GoDeprecation
	content, err := Marshal(envMap)
	if err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content + "\n")
	if err != nil {
		return err
	}
	return file.Sync()
}

// Marshal outputs the given environment as a dotenv-formatted environment file.
// Each line is in the format: KEY="VALUE" where VALUE is backslash-escaped.
//
// Deprecated: The serialization functions are untested and unmaintained.
func Marshal(envMap map[string]string) (string, error) {
	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		if d, err := strconv.Atoi(v); err == nil {
			lines = append(lines, fmt.Sprintf(`%s=%d`, k, d))
		} else {
			lines = append(lines, fmt.Sprintf(`%s="%s"`, k, doubleQuoteEscape(v))) // nolint // Cannot use %q here
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func filenamesOrDefault(filenames []string) []string {
	if len(filenames) == 0 {
		return []string{".env"}
	}
	return filenames
}

func loadFile(filename string, overload bool) error {
	envMap, err := readFile(filename, nil)
	if err != nil {
		return err
	}

	currentEnv := map[string]bool{}
	rawEnv := os.Environ()
	for _, rawEnvLine := range rawEnv {
		key := strings.Split(rawEnvLine, "=")[0]
		currentEnv[key] = true
	}

	for key, value := range envMap {
		if !currentEnv[key] || overload {
			_ = os.Setenv(key, value)
		}
	}

	return nil
}

func readFile(filename string, lookupFn LookupFn) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ParseWithLookup(file, lookupFn)
}

func expandVariables(value string, envMap map[string]string, lookupFn LookupFn) (string, error) {
	retVal, err := template.Substitute(value, func(k string) (string, bool) {
		if v, ok := envMap[k]; ok {
			return v, ok
		}
		return lookupFn(k)
	})
	if err != nil {
		return value, err
	}
	return retVal, nil
}

// Deprecated: only used by unsupported/untested code for Marshal/Write.
func doubleQuoteEscape(line string) string {
	const doubleQuoteSpecialChars = "\\\n\r\"!$`"
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.ReplaceAll(line, string(c), toReplace)
	}
	return line
}
