package catlib

type ISymbol interface {
	Name() string
	IsImportSymbol() bool
	IsExportSymbol() bool
}
