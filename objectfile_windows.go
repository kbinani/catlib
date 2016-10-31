package catlib

import (
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ObjectFile struct {
	IObjectFile
	filePath string
}

func (this *ObjectFile) Open(filePath string) error {
	this.filePath = filePath
	return nil
}

func (this *ObjectFile) RemoveDefaultlibDrectve(inputLibNames []string) (keptDefaultLibNames []string, err error) {
	peFile, e := pe.Open(this.filePath)
	keptDefaultLibNames = []string{}
	if e != nil {
		return []string{}, e
	}
	defer peFile.Close()

	reg := regexp.MustCompile(`-defaultlib:"[^"]*"`)
	r := regexp.MustCompile(`-defaultlib:"([^"]*)"`)

	section := peFile.Section(".drectve")
	if section == nil {
		return []string{}, nil
	}

	start := section.SectionHeader.Offset
	length := section.SectionHeader.Size
	data, err := section.Data()
	if err != nil {
		return []string{}, err
	}

	data = reg.ReplaceAllFunc(data, func(m []byte) []byte {
		rr := r.FindSubmatch(m)
		name := strings.ToLower(filepath.Base(string(rr[1])))
		ext := filepath.Ext(name)
		if ext != "" {
			name = strings.TrimSuffix(name, ext)
		}

		remove := false
		for _, n := range inputLibNames {
			if name == n {
				remove = true
				break
			}
		}

		if !remove && len(inputLibNames) > 0 {
			keptDefaultLibNames = append(keptDefaultLibNames, name)
			return m
		}

		for i := 0; i < len(m); i++ {
			m[i] = 0x20
		}
		return m
	})

	if len(data) != int(length) {
		return []string{}, fmt.Errorf("'.drectve' section length mismatch. expected %d for %d", length, len(data))
	}

	peFile.Close()

	file, e1 := os.OpenFile(this.filePath, os.O_RDWR, 0777)
	if e1 != nil {
		return []string{}, e1
	}
	defer file.Close()

	_, e2 := file.WriteAt(data, int64(start))
	if e2 != nil {
		return []string{}, e2
	}

	return keptDefaultLibNames, nil
}
