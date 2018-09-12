package analytics

import (
	"fmt"
	"os"
	"strings"

	"github.com/windmilleng/wmclient/pkg/dirs"
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

func OptStatus() (Opt, error) {
	txt, err := readChoiceFile()
	if err != nil {
		return OptDefault, err
	}

	switch txt {
	case Choices[OptIn]:
		return OptIn, nil
	case Choices[OptOut]:
		return OptOut, nil
	}

	return OptDefault, nil
}

func SetOptStr(s string) error {
	choice := OptDefault
	for k, v := range Choices {
		if v == s {
			choice = k
		}
		// allow "<appName> analytics opt in" to work
		if v == "opt-"+s {
			choice = k
		}
	}

	return SetOpt(choice)
}

func SetOpt(c Opt) error {
	s := c.String()

	d, err := dirs.UseWindmillDir()
	if err != nil {
		return err
	}

	if err = d.WriteFile(choiceFile, s); err != nil {
		return err
	}

	return nil
}

func readChoiceFile() (string, error) {
	d, err := dirs.UseWindmillDir()
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

func optedIn() bool {
	opt, err := OptStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "analytics.optedIn: %v\n", err)
	}

	return opt == OptIn
}
