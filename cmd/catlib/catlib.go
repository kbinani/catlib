package main

import (
	"fmt"
	. "github.com/kbinani/catlib"
	"github.com/ogier/pflag"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	objExt string
)

func init() {
	if runtime.GOOS == "windows" {
		objExt = ".obj"
	} else {
		objExt = ".o"
	}

	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)
}

func main() {
	start := time.Now()
	defer func() {
		os.RemoveAll(TempDir())
		elapsed := time.Since(start)
		fmt.Fprintf(os.Stderr, "Elapsed %s\n", elapsed)
	}()

	input := pflag.String("input", "", "comma separated list of file path of import libs")
	base := pflag.String("base", "", "file path of base static library")
	output := pflag.String("output", "", "file path of output library")
	arch := pflag.String("arch", "x86_64", "x86_64 or i386 (macOS only)")
	deleteDefaultLib := pflag.Bool("delete-default-lib", true, "delete '-defaultlib:\"libfoo\"' from '.drectve' section when libfoo.lib is in '--input' (Windows only)")
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

	if runtime.GOOS != "windows" {
		*deleteDefaultLib = false
	}

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

	keptLibNames := NewStringSet()

	importSyms := NewStringSet()

	work, _ := ioutil.TempDir(TempDir(), "objects")

	lastResolvedName := ""
	extracted := NewStringSet()
	index := 0

	// explode baseFile
	baseLib := new(LibFile)
	err := baseLib.Open(baseFile, *arch)
	if err != nil {
		panic(err)
	}
	defer baseLib.Close()

	m := new(sync.Mutex)
	var wg sync.WaitGroup

	for i := 0; i < baseLib.NumMembers(); i++ {
		if len(baseLib.ExportSymbols(i)) == 0 && len(baseLib.ImportSymbols(i)) == 0 {
			continue
		}
		name := fmt.Sprintf("b%d%s", index, objExt)
		p := filepath.Join(work, name)

		wg.Add(1)
		go func(i int, objectFile string) {
			defer wg.Done()

			f, err := os.Create(objectFile)
			if err != nil {
				panic(err)
			}
			if err := baseLib.Extract(i, f); err != nil {
				panic(err)
			}
			f.Close()

			// replace .drectve section
			if *deleteDefaultLib {
				var obj ObjectFile
				obj.Open(objectFile)
				names, err := obj.RemoveDefaultlibDrectve(inputLibNames)
				if err != nil {
					panic(err)
				}
				if len(names) > 0 {
					m.Lock()
					for _, name := range names {
						keptLibNames.Put(name)
					}
					m.Unlock()
				}
			}

			newname := fmt.Sprintf("%s%s", Sha256sum(objectFile), objExt)
			newp := filepath.Join(work, newname)
			for true {
				if err := os.Rename(objectFile, newp); err != nil {
					m.Lock()
					fmt.Fprintf(os.Stderr, "Info: retry renaming %s\n", objectFile)
					fmt.Fprintf(os.Stderr, "Reason:\n")
					fmt.Fprintf(os.Stderr, "\t%v\n", err)
					m.Unlock()
					continue
				}
				break
			}

			m.Lock()
			extracted.Put(newname)
			m.Unlock()
		}(i, p)

		index++

		for _, sym := range baseLib.ImportSymbols(i) {
			importSyms.Put(sym.Name())
		}
	}

	wg.Wait()

	libMap := openLibFiles(inputFiles, *arch)
	defer func() {
		for _, lib := range libMap {
			lib.Close()
		}
	}()

	alreadyResolvedSymbols := NewStringSet()
	alreadyExtractedFiles := NewStringSet()

	totalNumResolved := 0
	itr := 0
	for true {
		itr++
		numResolved := 0
		for k, inputFile := range inputFiles {
			lib := libMap[inputFile]

			for i := 0; i < lib.NumMembers(); i++ {
				name := fmt.Sprintf("%d_%d%s", k, i, objExt)
				if alreadyExtractedFiles.Has(name) {
					continue
				}

				exportSymbols := lib.ExportSymbols(i)
				if len(exportSymbols) == 0 {
					alreadyExtractedFiles.Put(name)
					continue
				}

				resolved := 0
				for _, sym := range exportSymbols {
					symName := sym.Name()
					if !importSyms.Has(symName) {
						continue
					}
					resolved++
					alreadyResolvedSymbols.Put(symName)
					importSyms.Del(symName)

					numResolved++
					totalNumResolved++

					name := fmt.Sprintf("%d:RESOLVED:%d:%s", itr, totalNumResolved, symName)
					if len(name) < len(lastResolvedName) {
						fmt.Printf("\r%s", strings.Repeat(" ", len(lastResolvedName)))
					}
					fmt.Printf("\r%s", name)
					lastResolvedName = name
				}

				if resolved == 0 {
					continue
				}

				alreadyExtractedFiles.Put(name)

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
					var obj ObjectFile
					obj.Open(p)
					names, err := obj.RemoveDefaultlibDrectve(inputLibNames)
					if err != nil {
						panic(err)
					}
					for _, name := range names {
						keptLibNames.Put(name)
					}
				}
				newname := fmt.Sprintf("%s%s", Sha256sum(p), objExt)
				newp := filepath.Join(work, newname)
				if err := os.Rename(p, newp); err != nil {
					panic(err)
				}

				extracted.Put(newname)

				for _, sym := range lib.ImportSymbols(i) {
					if alreadyResolvedSymbols.Has(sym.Name()) {
						continue
					}
					importSyms.Put(sym.Name())
				}
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

	if err := Concat(extracted.Values(), outputFile, work, *arch, *libflags); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	if *deleteDefaultLib && keptLibNames.Size() > 0 {
		fmt.Printf("These '-defaultlib:\"NAME\"' were not removed from '.drectve' section:\n")
		for _, name := range keptLibNames.Values() {
			fmt.Printf("  %s\n", name)
		}
	}
}

func openLibFiles(files []string, arch string) map[string]*LibFile {
	ret := make(map[string]*LibFile)
	var wg sync.WaitGroup
	m := new(sync.Mutex)

	for _, file := range files {
		wg.Add(1)
		go func(file, arch string) {
			defer wg.Done()

			lib := new(LibFile)
			err := lib.Open(file, arch)
			if err != nil {
				panic(err)
			}

			m.Lock()
			ret[file] = lib
			m.Unlock()
		}(file, arch)
	}

	wg.Wait()

	return ret
}
