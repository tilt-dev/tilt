package token

import (
	"os"

	"github.com/google/uuid"
	"github.com/tilt-dev/wmclient/pkg/dirs"
)

const tokenFileName = "token"

type Token string

func (t Token) String() string {
	return string(t)
}

func GetOrCreateToken(dir *dirs.WindmillDir) (Token, error) {
	token, err := getExistingToken(dir)
	if os.IsNotExist(err) {
		u := uuid.New()
		newtoken := Token(u.String())
		err := writeToken(dir, newtoken)
		if err != nil {
			return "", err
		}
		return newtoken, nil
	} else if err != nil {
		return "", err
	}

	return token, nil
}

func getExistingToken(dir *dirs.WindmillDir) (Token, error) {
	token, err := dir.ReadFile(tokenFileName)
	if err != nil {
		return "", err
	}
	return Token(token), nil
}

func writeToken(dir *dirs.WindmillDir, t Token) error {
	return dir.WriteFile(tokenFileName, string(t))
}
