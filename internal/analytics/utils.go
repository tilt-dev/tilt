package analytics

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
)

func HashMD5(s string) string {
	h := md5.Sum([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}

func HashSHA1(s string) string {
	h := sha1.Sum([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}
