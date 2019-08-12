package analytics

import (
	"crypto/md5"
	"encoding/base64"
)

func HashMD5(s string) string {
	h := md5.Sum([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}
