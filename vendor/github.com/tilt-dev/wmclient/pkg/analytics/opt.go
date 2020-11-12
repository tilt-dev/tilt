package analytics

import (
	"fmt"
	"os"
	"strings"

	"github.com/tilt-dev/wmclient/pkg/dirs"
)

type Opt int

const (
	OptDefault Opt = iota
	OptOut
	OptIn
)

var Choices = map[Opt]string{
	OptDefault: "default",
	OptOut:     "opt-out",
	OptIn:      "opt-in",
}

func (o Opt) String() string {
	val, ok := Choices[o]
	if ok {
		return val
	}
	return fmt.Sprintf("opt[%d]", o)
}

func ParseOpt(s string) (Opt, error) {
	for k, v := range Choices {
		// allow "<appName> analytics opt in" to work
		if s == v || fmt.Sprintf("opt-%s", s) == v {
			return k, nil
		}
	}

	return OptDefault, fmt.Errorf("unknown analytics opt: %q", s)
}

func OptStatus() (Opt, error) {
	txt, err := readChoiceFile()
	if err != nil {
		return OptDefault, err
	}

	// throw out invalid values
	opt, err := ParseOpt(txt)
	if err != nil {
		opt = OptDefault
	}
	return opt, nil
}

// SetOptStr converts the given string into an Opt enum, and records that choice
// as the users analytics decision, returning the Opt set (and any errors).
func SetOptStr(s string) (Opt, error) {
	choice, err := ParseOpt(s)
	if err != nil {
		return OptDefault, err
	}

	return choice, SetOpt(choice)
}

func SetOpt(c Opt) error {
	s := c.String()

	d, err := dirs.UseTiltDevDir()
	if err != nil {
		return err
	}

	if err = d.WriteFile(choiceFile, s); err != nil {
		return err
	}

	return nil
}

func readChoiceFile() (string, error) {
	d, err := dirs.UseTiltDevDir()
	if err != nil {
		return "", err
	}

	txt, err := d.ReadFile(choiceFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		txt = ""
	}

	return strings.TrimSpace(txt), nil
}

func optedIn() (bool, error) {
	opt, err := OptStatus()
	if err != nil {
		return false, fmt.Errorf("analytics.optedIn: %v", err)
	}

	return opt == OptIn, nil
}
