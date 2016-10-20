package catlib

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	IMAGE_SYM_CLASS_END_OF_FUNCTION  = 0x00ff
	IMAGE_SYM_CLASS_NULL             = 0x0000
	IMAGE_SYM_CLASS_AUTOMATIC        = 0x0001
	IMAGE_SYM_CLASS_EXTERNAL         = 0x0002
	IMAGE_SYM_CLASS_STATIC           = 0x0003
	IMAGE_SYM_CLASS_REGISTER         = 0x0004
	IMAGE_SYM_CLASS_EXTERNAL_DEF     = 0x0005
	IMAGE_SYM_CLASS_LABEL            = 0x0006
	IMAGE_SYM_CLASS_UNDEFINED_LABEL  = 0x0007
	IMAGE_SYM_CLASS_MEMBER_OF_STRUCT = 0x0008
	IMAGE_SYM_CLASS_ARGUMENT         = 0x0009
	IMAGE_SYM_CLASS_STRUCT_TAG       = 0x000A
	IMAGE_SYM_CLASS_MEMBER_OF_UNION  = 0x000B
	IMAGE_SYM_CLASS_UNION_TAG        = 0x000C
	IMAGE_SYM_CLASS_TYPE_DEFINITION  = 0x000D
	IMAGE_SYM_CLASS_UNDEFINED_STATIC = 0x000E
	IMAGE_SYM_CLASS_ENUM_TAG         = 0x000F
	IMAGE_SYM_CLASS_MEMBER_OF_ENUM   = 0x0010
	IMAGE_SYM_CLASS_REGISTER_PARAM   = 0x0011
	IMAGE_SYM_CLASS_BIT_FIELD        = 0x0012
	IMAGE_SYM_CLASS_FAR_EXTERNAL     = 0x0044
	IMAGE_SYM_CLASS_BLOCK            = 0x0064
	IMAGE_SYM_CLASS_FUNCTION         = 0x0065
	IMAGE_SYM_CLASS_END_OF_STRUCT    = 0x0066
	IMAGE_SYM_CLASS_FILE             = 0x0067
	IMAGE_SYM_CLASS_SECTION          = 0x0068
	IMAGE_SYM_CLASS_WEAK_EXTERNAL    = 0x0069
	IMAGE_SYM_CLASS_CLR_TOKEN        = 0x006B
)

type IMAGE_ARCHIVE_MEMBER_HEADER struct {
	RawName      [16]byte // 0
	RawDate      [12]byte // 16
	RawUserID    [6]byte  // 28
	RawGroupID   [6]byte  // 34
	RawMode      [8]byte  // 40
	RawSize      [10]byte // 48
	RawEndHeader [2]byte  // 58
}

type MemberHeader struct {
	ShortName  string
	Date       int
	UserID     int
	GroupID    int
	Mode       int
	Size       int
	LongName   string
	fileOffset int64
	symbols    []*pe.Symbol
}

type Lib struct {
	firstHeader      *MemberHeader
	secondHeader     *MemberHeader
	secondLinkMember tagSecondLinkerMember
	longNameHeader   *MemberHeader
	Members          []*MemberHeader
	filePath         string
}

type tagSecondLinkerMember struct {
	NumberOfMembers uint32
	Offsets         []uint32
	NumberOfSymbols uint32
	Indices         []uint16
	StringTable     []string
}

func (lib *Lib) Symbols(memberIndex int) []*pe.Symbol {
	return lib.Members[memberIndex].symbols
}

func isImportSymbol(symbol *pe.Symbol) bool {
	return symbol.StorageClass == IMAGE_SYM_CLASS_EXTERNAL && symbol.Value == 0 && symbol.SectionNumber == 0
}

func isExportSymbol(symbol *pe.Symbol) bool {
	return symbol.StorageClass == IMAGE_SYM_CLASS_EXTERNAL && (symbol.Value != 0 || symbol.SectionNumber != 0)
}

func (lib *Lib) ImportSymbols(memberIndex int) []*pe.Symbol {
	ret := []*pe.Symbol{}
	for _, sym := range lib.Members[memberIndex].symbols {
		if isImportSymbol(sym) {
			ret = append(ret, sym)
		}
	}

	// exclude symbols, which are resolved by the lib itself.
	result := []*pe.Symbol{}
	for _, sym := range ret {
		exported := false
		for _, m := range lib.Members {
			for _, s := range m.symbols {
				if sym.Name == s.Name && isExportSymbol(s) {
					exported = true
					break
				}
			}
			if exported {
				break
			}
		}
		if !exported {
			result = append(result, sym)
		}
	}
	return result
}

func (lib *Lib) ExportSymbols(memberIndex int) []*pe.Symbol {
	ret := []*pe.Symbol{}
	for _, sym := range lib.Members[memberIndex].symbols {
		if isExportSymbol(sym) {
			ret = append(ret, sym)
		}
	}
	return ret
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) name() string {
	return string(h.RawName[:len(h.RawName)])
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) size() int {
	s := string(h.RawSize[:len(h.RawSize)])
	s = strings.Trim(s, " ")
	i, _ := strconv.Atoi(s)
	return i
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) date() int {
	s := string(h.RawDate[:len(h.RawDate)])
	s = strings.Trim(s, " ")
	i, _ := strconv.Atoi(s)
	return i
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) userID() int {
	s := string(h.RawUserID[:len(h.RawUserID)])
	s = strings.Trim(s, " ")
	i, _ := strconv.Atoi(s)
	return i
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) groupID() int {
	s := string(h.RawGroupID[:len(h.RawGroupID)])
	s = strings.Trim(s, " ")
	i, _ := strconv.Atoi(s)
	return i
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) mode() int {
	s := string(h.RawMode[:len(h.RawMode)])
	i, _ := strconv.ParseInt(s, 8, 32)
	return int(i)
}

func (h IMAGE_ARCHIVE_MEMBER_HEADER) validate() error {
	endHeader := string(h.RawEndHeader[:len(h.RawEndHeader)])
	expectedEndHeader := "`\n"
	if endHeader != expectedEndHeader {
		return fmt.Errorf("invalid EndHeader: \"%s\" should be \"%s\"", endHeader, expectedEndHeader)
	}

	size := h.size()
	if size <= 0 {
		return fmt.Errorf("Size (%d) should be > 0", size)
	}

	return nil
}

func newMemberHeader(h *IMAGE_ARCHIVE_MEMBER_HEADER) *MemberHeader {
	m := new(MemberHeader)
	m.ShortName = h.name()
	m.Date = h.date()
	m.UserID = h.userID()
	m.GroupID = h.groupID()
	m.Mode = h.mode()
	m.Size = h.size()
	return m
}

func newImageArchiveMemberHeader(r io.Reader) (*MemberHeader, error) {
	h := new(IMAGE_ARCHIVE_MEMBER_HEADER)
	if err := binary.Read(r, binary.LittleEndian, h); err != nil {
		return nil, err
	}
	if err := h.validate(); err != nil {
		return nil, err
	}
	return newMemberHeader(h), nil
}

func newSecondLinkerMember(r io.Reader) (tagSecondLinkerMember, error) {
	var m tagSecondLinkerMember
	if err := binary.Read(r, binary.LittleEndian, &m.NumberOfMembers); err != nil {
		return m, err
	}
	m.Offsets = make([]uint32, m.NumberOfMembers)
	for i := 0; i < int(m.NumberOfMembers); i++ {
		if err := binary.Read(r, binary.LittleEndian, &m.Offsets[i]); err != nil {
			return m, err
		}
	}
	if err := binary.Read(r, binary.LittleEndian, &m.NumberOfSymbols); err != nil {
		return m, err
	}
	m.Indices = make([]uint16, m.NumberOfSymbols)
	m.StringTable = make([]string, m.NumberOfSymbols)
	for i := 0; i < int(m.NumberOfSymbols); i++ {
		if err := binary.Read(r, binary.LittleEndian, &m.Indices[i]); err != nil {
			return m, err
		}
	}
	buffer := []byte{}
	index := 0
	for index < int(m.NumberOfSymbols) {
		var c byte
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			return m, err
		}
		if c == byte(0) {
			m.StringTable[index] = string(buffer)
			buffer = make([]byte, 0)
			index++
		} else {
			buffer = append(buffer, c)
		}
	}
	return m, nil
}

func NewLib(filePath string) (lib *Lib, err error) {
	lib = new(Lib)
	lib.filePath = filePath
	r, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	magicBytes := make([]byte, 8)
	n, e := r.Read(magicBytes)
	if n != len(magicBytes) || e != nil {
		return nil, e
	}
	magic := string(magicBytes[:])
	expectedMagic := "!<arch>\n"
	if magic != expectedMagic {
		return nil, fmt.Errorf("invalid magic header: \"%s\" should be \"%s\"", magic, expectedMagic)
	}

	lib.firstHeader, err = newImageArchiveMemberHeader(r)
	if err != nil {
		return nil, err
	}

	// skip first archive member header, and seek to second archive member header.
	var pos int64
	pos, err = r.Seek(int64(lib.firstHeader.Size), io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// IMAGE_ARCHIVE_MEMBER_HEADER should have been placed 2byte padding.
	if pos%2 == 1 {
		pos, err = r.Seek(1, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
	}

	lib.secondHeader, err = newImageArchiveMemberHeader(r)
	if err != nil {
		return nil, err
	}

	// read second archive member header.
	lib.secondLinkMember, err = newSecondLinkerMember(r)

	// read long-name member table if exist
	buffer := []byte{}
	p, _ := r.Seek(0, io.SeekCurrent)
	if p%2 == 1 {
		r.Seek(1, io.SeekCurrent)
	}
	h, e := newImageArchiveMemberHeader(r)
	if e == nil && h.ShortName == "//              " {
		lib.longNameHeader = h
		buffer = make([]byte, lib.longNameHeader.Size)
		_, err = r.Read(buffer)
		if err != nil {
			return nil, err
		}
	}

	for i := 0; i < int(lib.secondLinkMember.NumberOfMembers); i++ {
		_, err := r.Seek(int64(lib.secondLinkMember.Offsets[i]), io.SeekStart)
		if err != nil {
			return nil, err
		}
		m, e := newImageArchiveMemberHeader(r)
		if e != nil {
			return nil, e
		}
		m.fileOffset, _ = r.Seek(0, io.SeekCurrent)
		if strings.HasPrefix(m.ShortName, "/") {
			offsetStr := strings.TrimRight(m.ShortName[1:], " ")
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				return nil, err
			}
			m.LongName = stringPart(offset, &buffer)
		}

		limitReader := io.NewSectionReader(r, m.fileOffset, int64(m.Size))
		obj, e1 := pe.NewFile(limitReader)
		if e1 != nil {
			continue
		}
		for _, sym := range obj.Symbols {
			m.symbols = append(m.symbols, sym)
		}

		lib.Members = append(lib.Members, m)
	}

	return lib, nil
}

func stringPart(offset int, buffer *[]byte) string {
	for i := offset; i < len(*buffer); i++ {
		c := (*buffer)[i]
		if c == 0 {
			return string((*buffer)[offset:i])
		}
	}
	return ""
}

func (m *MemberHeader) Name() string {
	if m.LongName != "" {
		return m.LongName
	}
	return strings.TrimRight(strings.TrimRight(m.ShortName, " "), "/")
}

func (lib *Lib) Extract(memberIndex int, w io.Writer) error {
	file, err := os.Open(lib.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	m := lib.Members[memberIndex]
	_, err = file.Seek(m.fileOffset, io.SeekStart)
	limitReader := io.LimitReader(file, int64(m.Size))
	n, e := io.Copy(w, limitReader)
	if e != nil {
		return e
	}
	if n != int64(m.Size) {
		return fmt.Errorf("Written file size mismatch expected %d for %d", m.Size, n)
	}

	return nil
}
