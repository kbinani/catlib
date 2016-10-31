package catlib

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
)

var (
	tempDir string
)

func init() {
	tempDir, _ = ioutil.TempDir(os.TempDir(), "catlib_")
}

func IsFileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	return false
}

func Sha256sum(filePath string) string {
	h := sha256.New()
	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TempDir() string {
	return tempDir
}
