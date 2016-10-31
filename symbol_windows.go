package catlib

import (
	"debug/pe"
)

type Symbol struct {
	ISymbol
	symbol *pe.Symbol
}

func NewSymbol(symbol *pe.Symbol) Symbol {
	var s Symbol
	s.symbol = symbol
	return s
}

func (this *Symbol) IsImportSymbol() bool {
	return this.symbol.StorageClass == IMAGE_SYM_CLASS_EXTERNAL && this.symbol.Value == 0 && this.symbol.SectionNumber == 0
}

func (this *Symbol) IsExportSymbol() bool {
	return this.symbol.StorageClass == IMAGE_SYM_CLASS_EXTERNAL && (this.symbol.Value != 0 || this.symbol.SectionNumber != 0)
}

func (this *Symbol) Name() string {
	return this.symbol.Name
}
