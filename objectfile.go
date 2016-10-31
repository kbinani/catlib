package catlib

type IObjectFile interface {
	Open(filePath string) error
	RemoveDefaultlibDrectve(inputLibNames []string) (keptDefaultLibNames []string, err error)
}
