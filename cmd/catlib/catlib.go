package main

import (
	"bufio"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"github.com/kbinani/catlib"
	"github.com/ogier/pflag"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	lib = ""
)

func init() {
	_, vscomntools := findVSComnTools()
	if vscomntools == "" {
		return
	}
	binDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(vscomntools))), "VC", "bin")

	lib = filepath.Join(binDir, "lib.exe")
}

func main() {
	input := pflag.String("input", "", "comma separated list of file path of import libs")
	base := pflag.String("base", "", "file path of base static library")
	output := pflag.String("output", "", "file path of output library")
	deleteDefaultLib := pflag.Bool("delete-default-lib", true, "delete '-defaultlib:\"libfoo\"' from '.drectve' section when libfoo.lib is in '--input'")
	libflags := pflag.String("extra-lib-flags", "", "extra 'lib' command options for final concatenation stage")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", filepath.Base(os.Args[0]))
		pflag.PrintDefaults()

		lines := []string{
			"Example:",
			"  catlib --base=myproject.lib ^",
			"         --input=zlibstat.lib,libprotobuf.lib ^",
			"         --output=myproject-prelinked.lib ^",
			"         --delete-default-lib ^",
			"         --extra-lib-flags=\"/LTCG /WX\"",
		}
		for _, line := range lines {
			fmt.Fprintf(os.Stderr, "%s\n", line)
		}
	}

	pflag.Parse()

	if !isFileExist(lib) {
		fmt.Fprintf(os.Stderr, "lib command not found\n")
		return
	}

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Fprintf(os.Stderr, "Elapsed %s\n", elapsed)
	}()

	inputFiles := strings.Split(*input, ",")
	baseFile, _ := filepath.Abs(*base)
	outputFile, _ := filepath.Abs(*output)

	inputLibNames := []string{}
	for _, inputFile := range inputFiles {
		name := strings.ToLower(filepath.Base(inputFile))
		ext := filepath.Ext(name)
		if ext != "" {
			name = strings.TrimSuffix(name, ext)
		}
		inputLibNames = append(inputLibNames, name)
	}

	keptLibNames := []string{}

	importSyms := []string{}

	work, _ := ioutil.TempDir(os.TempDir(), "concatlib_")
	defer os.RemoveAll(work)

	lastResolvedName := ""
	extracted := []string{}
	index := 0

	// explode baseFile
	baseLib, err := catlib.NewLib(baseFile)
	if err != nil {
		panic(err)
	}
	for i := range baseLib.Members {
		name := fmt.Sprintf("b%d.obj", index)
		p := filepath.Join(work, name)
		f, err := os.Create(p)
		if err != nil {
			panic(err)
		}
		if err := baseLib.Extract(i, f); err != nil {
			panic(err)
		}
		f.Close()

		// replace .drectve section
		if *deleteDefaultLib {
			names, err := removeDefaultlibDrectve(p, inputLibNames)
			if err != nil {
				panic(err)
			}
			for _, name := range names {
				keptLibNames = append(keptLibNames, name)
			}
			keptLibNames = uniqueString(keptLibNames)
		}

		newname := fmt.Sprintf("%s.obj", sha256sum(p))
		newp := filepath.Join(work, newname)
		if err := os.Rename(p, newp); err != nil {
			panic(err)
		}

		extracted = append(extracted, newname)
		index++

		for _, sym := range baseLib.ImportSymbols(i) {
			importSyms = append(importSyms, sym.Name)
		}
	}

	libMap := make(map[string]*catlib.Lib)
	for _, inputFile := range inputFiles {
		lib, err := catlib.NewLib(inputFile)
		if err != nil {
			panic(err)
		}
		libMap[inputFile] = lib
	}

	alreadyResolvedSymbols := make(map[string]int)
	alreadyExtractedFiles := make(map[string]int)

	totalNumResolved := 0
	itr := 0
	for true {
		itr++
		numResolved := 0
		for k, inputFile := range inputFiles {
			lib := libMap[inputFile]

			for i := range lib.Members {
				name := fmt.Sprintf("%d_%d.obj", k, i)
				_, ok := alreadyExtractedFiles[name]
				if ok {
					continue
				}

				symbols := []string{}
				for _, sym := range lib.ExportSymbols(i) {
					symbols = append(symbols, sym.Name)
				}

				resolved := []string{}
				changed := true
				for changed {
					changed = false
					for k, symImport := range importSyms {
						j := -1
						_, ok := alreadyResolvedSymbols[symImport]
						if ok {
							continue
						}
						for _, sym := range symbols {
							if symImport == sym {
								changed = true
								resolved = append(resolved, symImport)
								numResolved++
								totalNumResolved++

								name := fmt.Sprintf("%d:RESOLVED:%d:%s", itr, totalNumResolved, sym)
								if len(name) < len(lastResolvedName) {
									fmt.Printf("\r%s", strings.Repeat(" ", len(lastResolvedName)))
								}
								fmt.Printf("\r%s", name)
								lastResolvedName = name
								j = k
								break
							}
						}
						if j < 0 {
							continue
						}
						alreadyResolvedSymbols[symImport] = 0
						swap := importSyms
						importSyms = make([]string, 0)
						for l, sym := range swap {
							if l != j {
								importSyms = append(importSyms, sym)
							}
						}
						break
					}
				}
				if len(resolved) == 0 {
					continue
				}

				alreadyExtractedFiles[name] = 0

				p := filepath.Join(work, name)
				f, err := os.Create(p)
				if err != nil {
					panic(err)
				}
				if err := lib.Extract(i, f); err != nil {
					panic(err)
				}
				f.Close()

				if *deleteDefaultLib {
					names, err := removeDefaultlibDrectve(p, inputLibNames)
					if err != nil {
						panic(err)
					}
					for _, name := range names {
						keptLibNames = append(keptLibNames, name)
					}
					keptLibNames = uniqueString(keptLibNames)
				}
				newname := fmt.Sprintf("%s.obj", sha256sum(p))
				newp := filepath.Join(work, newname)
				if err := os.Rename(p, newp); err != nil {
					panic(err)
				}

				extracted = append(extracted, newname)

				for _, sym := range lib.Symbols(i) {
					if isImportSymbol(sym) {
						importSyms = append(importSyms, sym.Name)
					}
				}
				importSyms = uniqueString(importSyms)
			}
		}
		if numResolved == 0 {
			break
		}
	}

	if totalNumResolved == 0 {
		fmt.Printf("ABORT: No symbol resolved\n")
		return
	} else {
		fmt.Printf("\n")
	}

	extracted = uniqueString(extracted)

	if err := concat(extracted, outputFile, work, *libflags); err != nil {
		fmt.Fprintf(os.Stderr, "'lib' command failed: %v\n", err)
		return
	}
	if *deleteDefaultLib && len(keptLibNames) > 0 {
		fmt.Printf("These '-defaultlib:\"NAME\"' were not removed from '.drectve' section:\n")
		for _, name := range keptLibNames {
			fmt.Printf("  %s\n", name)
		}
	}
}

func sha256sum(filePath string) string {
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

func removeDefaultlibDrectve(filePath string, inputLibNames []string) (keptDefaultLibNames []string, err error) {
	peFile, e := pe.Open(filePath)
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

	file, e1 := os.OpenFile(filePath, os.O_RDWR, 0777)
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

func uniqueString(input []string) []string {
	ret := []string{}
	for _, s := range input {
		found := false
		for _, i := range ret {
			if i == s {
				found = true
				break
			}
		}
		if !found {
			ret = append(ret, s)
		}
	}
	return ret
}

func concat(files []string, output, workingDirectory, libflags string) error {
	fp, err := ioutil.TempFile(os.TempDir(), "concatlib_")
	if err != nil {
		return err
	}
	defer os.Remove(fp.Name())

	fmt.Fprintf(fp, "%s\n", libflags)
	for _, file := range files {
		fmt.Fprintf(fp, "\"%s\"\n", file)
	}
	fmt.Fprintf(fp, "/OUT:\"%s\"\n", output)
	fp.Close()

	c := exec.Command(lib, fmt.Sprintf("@%s", fp.Name()), "/NOLOGO")
	c.Dir = workingDirectory
	stdout, err := catlib.StdoutPipe(c)
	c.Start()

	s := bufio.NewScanner(stdout)
	for s.Scan() {
		fmt.Printf("%s\n", s.Text())
	}
	err = c.Wait()

	if err != nil {
		return fmt.Errorf("Err: %v, Args: %v", err, c.Args)
	}
	return nil
}

func isImportSymbol(symbol *pe.Symbol) bool {
	return symbol.StorageClass == catlib.IMAGE_SYM_CLASS_EXTERNAL && symbol.Value == 0 && symbol.SectionNumber == 0
}

func findVSComnTools() (name, value string) {
	reg := regexp.MustCompile(`^VS([0-9]*)COMNTOOLS$`)
	comtools := make(map[int]string)
	for _, e := range os.Environ() {
		tokens := strings.Split(e, "=")
		if len(tokens) != 2 {
			continue
		}
		key := tokens[0]
		result := reg.FindSubmatch([]byte(key))
		if len(result) == 0 {
			continue
		}
		version, err := strconv.Atoi(string(result[1]))
		if err != nil {
			continue
		}
		if version <= 0 {
			continue
		}
		value := tokens[1]
		comtools[version] = value
	}

	maxVersion := -1
	for version := range comtools {
		if maxVersion < version {
			maxVersion = version
		}
	}

	if maxVersion == -1 {
		return "", ""
	}
	return fmt.Sprintf("VS%dCOMNTOOLS", maxVersion), comtools[maxVersion]
}

func isFileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	return false
}
