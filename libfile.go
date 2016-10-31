package catlib

import (
	"io"
)

type ILibFile interface {
	Open(filePath string, arch string) error
	Close()
	NumMembers() int
	Extract(memberIndex int, w io.Writer) error
	ImportSymbols(memberIndex int) []ISymbol
	ExportSymbols(memberIndex int) []ISymbol
}
