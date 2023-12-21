package util

import (
	"crypto/md5"
	"encoding/hex"
)

func GetTagFromFilename(filename string) string {
	sum := md5.Sum([]byte(filename))
	return hex.EncodeToString(sum[:])
}
