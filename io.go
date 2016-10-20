package catlib

import (
	"bufio"
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
)

var enc *encoding.Encoding

func detectEncoding() (*encoding.Encoding, error) {
	if runtime.GOOS == `windows` {
		c := exec.Command("chcp")
		out, e2 := c.StdoutPipe()
		if e2 != nil {
			return nil, e2
		}
		c.Start()
		s := bufio.NewScanner(out)
		codepage := -1
		reg := regexp.MustCompile(`^.*\s([0-9]*)\s*$`)
		for s.Scan() {
			line := s.Text()
			if len(line) == 0 {
				continue
			}
			m := reg.FindSubmatch([]byte(line))
			if len(m) < 2 {
				continue
			}
			codepageStr := string(m[1])
			i, e4 := strconv.Atoi(codepageStr)
			if e4 != nil {
				continue
			}
			codepage = i
		}
		e3 := c.Wait()
		if e3 != nil {
			return nil, e3
		}

		switch codepage {
		case 932:
			return &japanese.ShiftJIS, nil
		case 20932:
			return &japanese.EUCJP, nil
		case 50220, 50221, 50222:
			return &japanese.ISO2022JP, nil
		case 949:
			return &korean.EUCKR, nil
		case 54936:
			return &simplifiedchinese.GB18030, nil
		case 936:
			return &simplifiedchinese.GBK, nil
		case 52936:
			return &simplifiedchinese.HZGB2312, nil
		case 950:
			return &traditionalchinese.Big5, nil
		}

		return nil, fmt.Errorf("cannot detect encoding")
	} else {
		return &unicode.UTF8, nil
	}
}

func currentEncoding() encoding.Encoding {
	if enc == nil {
		ret, err := detectEncoding()
		if err != nil {
			enc = &unicode.UTF8
		} else {
			enc = ret
		}
	}
	return *enc
}

func StdoutPipe(cmd *exec.Cmd) (io.Reader, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ret := transform.NewReader(stdout, currentEncoding().NewDecoder())
	return ret, nil
}

func StderrPipe(cmd *exec.Cmd) (io.Reader, error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	ret := transform.NewReader(stderr, currentEncoding().NewDecoder())
	return ret, nil
}
