package catlib

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type LibFile struct {
	ILibFile
	filePath          string
	members           []libMember
	extractedFilePath string
	tempDir           string
}

type libMember struct {
	ImportSymbols []Symbol
	ExportSymbols []Symbol
	Name          string
}

func (this *LibFile) Open(filePath string, arch string) (err error) {
	this.filePath = filePath
	this.tempDir, err = ioutil.TempDir(TempDir(), "lib")
	if err != nil {
		return err
	}
	file, e := ioutil.TempFile(TempDir(), "extracted")
	if e != nil {
		return e
	}
	this.extractedFilePath = file.Name()
	err = extractFatBinary(this.filePath, this.extractedFilePath, arch)
	if err != nil {
		return err
	}

	err = extractAllMembers(this.extractedFilePath, this.tempDir)

	regObjectName := regexp.MustCompile(`^.*\((.*)\):$`)
	regExportFunction := regexp.MustCompile(`^([0-9A-Za-z]+)?\s+(T|U|S|D|t|s|d)\s+(.*)$`)
	cmd := exec.Command("nm", "-arch", arch, filePath)
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	cmd.Start()
	s := bufio.NewScanner(stdout)
	var current *libMember = nil
	for s.Scan() {
		line := []byte(s.Text())
		if len(line) == 0 {
			continue
		}
		match := regObjectName.FindSubmatch(line)
		if len(match) < 2 {
			if current == nil {
				continue
			}
			m := regExportFunction.FindSubmatch(line)
			if len(m) < 4 {
				continue
			}
			t := string(m[2])
			funcName := string(m[3])
			if t == "U" || t == "t" || t == "s" || t == "d" {
				current.ImportSymbols = append(current.ImportSymbols, NewSymbol(funcName, true))
			} else {
				current.ExportSymbols = append(current.ExportSymbols, NewSymbol(funcName, false))
			}
		} else {
			name := string(match[1])
			if current != nil {
				this.members = append(this.members, *current)
			}
			current = new(libMember)
			current.Name = name
		}
	}
	if current != nil {
		this.members = append(this.members, *current)
	}

	for _, m := range this.members {
		actuallyImported := []Symbol{}
		for _, im := range m.ImportSymbols {
			found := false
			for _, ex := range m.ExportSymbols {
				if ex.Name() == im.Name() {
					found = true
					break
				}
			}
			if !found {
				actuallyImported = append(actuallyImported, im)
			}
		}
		m.ImportSymbols = actuallyImported
	}

	return nil
}

func extractAllMembers(filePath string, directory string) error {
	cmd := exec.Command("ar", "-x", filePath)
	cmd.Dir = directory
	bytes, e := cmd.CombinedOutput()
	if e != nil {
		fmt.Printf("%s", string(bytes))
		return e
	}
	return nil
}

func (this *LibFile) Close() {
	os.Remove(this.extractedFilePath)
	os.RemoveAll(this.tempDir)
}

func (this *LibFile) NumMembers() int {
	return len(this.members)
}

func (this *LibFile) Extract(memberIndex int, w io.Writer) error {
	memberName := this.members[memberIndex].Name
	srcPath := filepath.Join(this.tempDir, memberName)
	src, e1 := os.Open(srcPath)
	if e1 != nil {
		return e1
	}
	defer src.Close()
	_, e2 := io.Copy(w, src)
	if e2 != nil {
		return e2
	}
	return nil
}

func (this *LibFile) ImportSymbols(memberIndex int) []Symbol {
	return this.members[memberIndex].ImportSymbols
}

func (this *LibFile) ExportSymbols(memberIndex int) []Symbol {
	return this.members[memberIndex].ExportSymbols
}

func Concat(files []string, output, workingDirectory, arch, libflags string) error {
	filelist, err := ioutil.TempFile(TempDir(), "libtool")
	if err != nil {
		return err
	}
	fp, err := os.Create(filelist.Name())
	for _, f := range files {
		fmt.Fprintf(fp, "%s\n", f)
	}
	fp.Close()
	defer os.Remove(filelist.Name())

	cmd := exec.Command("libtool")
	cmd.Args = append(cmd.Args, "-static")
	cmd.Args = append(cmd.Args, "-arch_only")
	cmd.Args = append(cmd.Args, arch)
	cmd.Args = append(cmd.Args, "-filelist")
	cmd.Args = append(cmd.Args, filelist.Name())
	cmd.Args = append(cmd.Args, "-o")
	cmd.Args = append(cmd.Args, output)
	cmd.Dir = workingDirectory
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return err
}

func extractFatBinary(inFilePath string, outFilePath string, arch string) error {
	cmd := exec.Command("libtool", "-static", "-arch_only", arch, inFilePath, "-o", outFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, string(output))
		return err
	}
	return nil
}
