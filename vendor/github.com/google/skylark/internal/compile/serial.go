package compile

// This files defines functions to read and write a compile.Program to a file.
// Currently we use gob encoding because it is convenient.
//
// It is the client's responsibility to manage version skew between the
// compiler used to produce a file and the interpreter that consumes it.
// The version number is provided as a constant. Incompatible protocol
// changes should also increment the version number.

import (
	"encoding/gob"
	"fmt"
	"io"

	"github.com/google/skylark/syntax"
)

const magic = "!sky"

type gobProgram struct {
	Version   int
	Filename  string
	Loads     []gobIdent
	Names     []string
	Constants []interface{}
	Functions []gobFunction
	Globals   []gobIdent
	Toplevel  gobFunction
}

type gobFunction struct {
	Id                    gobIdent // hack: name and pos
	Code                  []byte
	Pclinetab             []uint16
	Locals                []gobIdent
	Freevars              []gobIdent
	MaxStack              int
	NumParams             int
	HasVarargs, HasKwargs bool
}

type gobIdent struct {
	Name      string
	Line, Col int32 // the filename is gobProgram.Filename
}

// Write writes a compiled Skylark program to out.
func (prog *Program) Write(out io.Writer) error {
	out.Write([]byte(magic))

	gobIdents := func(idents []Ident) []gobIdent {
		res := make([]gobIdent, len(idents))
		for i, id := range idents {
			res[i].Name = id.Name
			res[i].Line = id.Pos.Line
			res[i].Col = id.Pos.Col
		}
		return res
	}

	gobFunc := func(fn *Funcode) gobFunction {
		return gobFunction{
			Id: gobIdent{
				Name: fn.Name,
				Line: fn.Pos.Line,
				Col:  fn.Pos.Col,
			},
			Code:       fn.Code,
			Pclinetab:  fn.pclinetab,
			Locals:     gobIdents(fn.Locals),
			Freevars:   gobIdents(fn.Freevars),
			MaxStack:   fn.MaxStack,
			NumParams:  fn.NumParams,
			HasVarargs: fn.HasVarargs,
			HasKwargs:  fn.HasKwargs,
		}
	}

	gp := &gobProgram{
		Version:   Version,
		Filename:  prog.Toplevel.Pos.Filename(),
		Loads:     gobIdents(prog.Loads),
		Names:     prog.Names,
		Constants: prog.Constants,
		Functions: make([]gobFunction, len(prog.Functions)),
		Globals:   gobIdents(prog.Globals),
		Toplevel:  gobFunc(prog.Toplevel),
	}
	for i, f := range prog.Functions {
		gp.Functions[i] = gobFunc(f)
	}

	return gob.NewEncoder(out).Encode(gp)
}

// ReadProgram reads a compiled Skylark program from in.
func ReadProgram(in io.Reader) (*Program, error) {
	magicBuf := []byte(magic)
	n, err := in.Read(magicBuf)
	if err != nil {
		return nil, err
	}
	if n != len(magic) {
		return nil, fmt.Errorf("not a compiled module: no magic number")
	}
	if string(magicBuf) != magic {
		return nil, fmt.Errorf("not a compiled module: got magic number %q, want %q",
			magicBuf, magic)
	}

	dec := gob.NewDecoder(in)
	var gp gobProgram
	if err := dec.Decode(&gp); err != nil {
		return nil, fmt.Errorf("decoding program: %v", err)
	}

	if gp.Version != Version {
		return nil, fmt.Errorf("version mismatch: read %d, want %d",
			gp.Version, Version)
	}

	file := gp.Filename // copy, to avoid keeping gp live

	ungobIdents := func(idents []gobIdent) []Ident {
		res := make([]Ident, len(idents))
		for i, id := range idents {
			res[i].Name = id.Name
			res[i].Pos = syntax.MakePosition(&file, id.Line, id.Col)
		}
		return res
	}

	prog := &Program{
		Loads:     ungobIdents(gp.Loads),
		Names:     gp.Names,
		Constants: gp.Constants,
		Globals:   ungobIdents(gp.Globals),
		Functions: make([]*Funcode, len(gp.Functions)),
	}

	ungobFunc := func(gf *gobFunction) *Funcode {
		pos := syntax.MakePosition(&file, gf.Id.Line, gf.Id.Col)
		return &Funcode{
			Prog:       prog,
			Pos:        pos,
			Name:       gf.Id.Name,
			Code:       gf.Code,
			pclinetab:  gf.Pclinetab,
			Locals:     ungobIdents(gf.Locals),
			Freevars:   ungobIdents(gf.Freevars),
			MaxStack:   gf.MaxStack,
			NumParams:  gf.NumParams,
			HasVarargs: gf.HasVarargs,
			HasKwargs:  gf.HasKwargs,
		}
	}

	for i := range gp.Functions {
		prog.Functions[i] = ungobFunc(&gp.Functions[i])
	}
	prog.Toplevel = ungobFunc(&gp.Toplevel)
	return prog, nil
}
