package catlib

type Symbol struct {
	ISymbol
	name      string
	undefined bool
}

func NewSymbol(name string, undefined bool) Symbol {
	var this Symbol
	this.name = name
	this.undefined = undefined
	return this
}

func (this *Symbol) Name() string {
	return this.name
}

func (this *Symbol) IsImportSymbol() bool {
	return this.undefined
}

func (this *Symbol) IsExportSymbol() bool {
	return !this.undefined
}
