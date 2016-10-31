package catlib

type ObjectFile struct {
	IObjectFile
}

func (this *ObjectFile) Open(filePath string) error {
	return nil
}

func (this *ObjectFile) RemoveDefaultlibDrectve(inputLibNames []string) (keptDefaultLibNames []string, err error) {
	return []string{}, nil
}
