// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package skylark

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/skylark/internal/compile"
	"github.com/google/skylark/resolve"
	"github.com/google/skylark/syntax"
)

const debug = false

// A Thread contains the state of a Skylark thread,
// such as its call stack and thread-local storage.
// The Thread is threaded throughout the evaluator.
type Thread struct {
	// frame is the current Skylark execution frame.
	frame *Frame

	// Print is the client-supplied implementation of the Skylark
	// 'print' function. If nil, fmt.Fprintln(os.Stderr, msg) is
	// used instead.
	Print func(thread *Thread, msg string)

	// Load is the client-supplied implementation of module loading.
	// Repeated calls with the same module name must return the same
	// module environment or error.
	// The error message need not include the module name.
	//
	// See example_test.go for some example implementations of Load.
	Load func(thread *Thread, module string) (StringDict, error)

	// locals holds arbitrary "thread-local" Go values belonging to the client.
	// They are accessible to the client but not to any Skylark program.
	locals map[string]interface{}
}

// SetLocal sets the thread-local value associated with the specified key.
// It must not be called after execution begins.
func (thread *Thread) SetLocal(key string, value interface{}) {
	if thread.locals == nil {
		thread.locals = make(map[string]interface{})
	}
	thread.locals[key] = value
}

// Local returns the thread-local value associated with the specified key.
func (thread *Thread) Local(key string) interface{} {
	return thread.locals[key]
}

// Caller returns the frame of the caller of the current function.
// It should only be used in built-ins called from Skylark code.
func (thread *Thread) Caller() *Frame { return thread.frame.parent }

// TopFrame returns the topmost stack frame.
func (thread *Thread) TopFrame() *Frame { return thread.frame }

// A StringDict is a mapping from names to values, and represents
// an environment such as the global variables of a module.
// It is not a true skylark.Value.
type StringDict map[string]Value

func (d StringDict) String() string {
	names := make([]string, 0, len(d))
	for name := range d {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	path := make([]Value, 0, 4)
	buf.WriteByte('{')
	sep := ""
	for _, name := range names {
		buf.WriteString(sep)
		buf.WriteString(name)
		buf.WriteString(": ")
		writeValue(&buf, d[name], path)
		sep = ", "
	}
	buf.WriteByte('}')
	return buf.String()
}

func (d StringDict) Freeze() {
	for _, v := range d {
		v.Freeze()
	}
}

// Has reports whether the dictionary contains the specified key.
func (d StringDict) Has(key string) bool { _, ok := d[key]; return ok }

// A Frame records a call to a Skylark function (including module toplevel)
// or a built-in function or method.
type Frame struct {
	parent   *Frame          // caller's frame (or nil)
	callable Callable        // current function (or toplevel) or built-in
	posn     syntax.Position // source position of PC, set during error
	callpc   uint32          // PC of position of active call, set during call
}

// The Frames of a thread are structured as a spaghetti stack, not a
// slice, so that an EvalError can copy a stack efficiently and immutably.
// In hindsight using a slice would have led to a more convenient API.

func (fr *Frame) errorf(posn syntax.Position, format string, args ...interface{}) *EvalError {
	fr.posn = posn
	msg := fmt.Sprintf(format, args...)
	return &EvalError{Msg: msg, Frame: fr}
}

// Position returns the source position of the current point of execution in this frame.
func (fr *Frame) Position() syntax.Position {
	if fr.posn.IsValid() {
		return fr.posn // leaf frame only (the error)
	}
	if fn, ok := fr.callable.(*Function); ok {
		return fn.funcode.Position(fr.callpc) // position of active call
	}
	return syntax.MakePosition(&builtinFilename, 1, 0)
}

var builtinFilename = "<builtin>"

// Function returns the frame's function or built-in.
func (fr *Frame) Callable() Callable { return fr.callable }

// Parent returns the frame of the enclosing function call, if any.
func (fr *Frame) Parent() *Frame { return fr.parent }

// An EvalError is a Skylark evaluation error and its associated call stack.
type EvalError struct {
	Msg   string
	Frame *Frame
}

func (e *EvalError) Error() string { return e.Msg }

// Backtrace returns a user-friendly error message describing the stack
// of calls that led to this error.
func (e *EvalError) Backtrace() string {
	var buf bytes.Buffer
	e.Frame.WriteBacktrace(&buf)
	fmt.Fprintf(&buf, "Error: %s", e.Msg)
	return buf.String()
}

// WriteBacktrace writes a user-friendly description of the stack to buf.
func (fr *Frame) WriteBacktrace(out *bytes.Buffer) {
	fmt.Fprintf(out, "Traceback (most recent call last):\n")
	var print func(fr *Frame)
	print = func(fr *Frame) {
		if fr != nil {
			print(fr.parent)
			fmt.Fprintf(out, "  %s: in %s\n", fr.Position(), fr.Callable().Name())
		}
	}
	print(fr)
}

// Stack returns the stack of frames, innermost first.
func (e *EvalError) Stack() []*Frame {
	var stack []*Frame
	for fr := e.Frame; fr != nil; fr = fr.parent {
		stack = append(stack, fr)
	}
	return stack
}

// A Program is a compiled Skylark program.
//
// Programs are immutable, and contain no Values.
// A Program may be created by parsing a source file (see SourceProgram)
// or by loading a previously saved compiled program (see CompiledProgram).
type Program struct {
	compiled *compile.Program
}

// CompilerVersion is the version number of the protocol for compiled
// files. Applications must not run programs compiled by one version
// with an interpreter at another version, and should thus incorporate
// the compiler version into the cache key when reusing compiled code.
const CompilerVersion = compile.Version

// NumLoads returns the number of load statements in the compiled program.
func (prog *Program) NumLoads() int { return len(prog.compiled.Loads) }

// Load(i) returns the name and position of the i'th module directly
// loaded by this one, where 0 <= i < NumLoads().
// The name is unresolved---exactly as it appears in the source.
func (prog *Program) Load(i int) (string, syntax.Position) {
	id := prog.compiled.Loads[i]
	return id.Name, id.Pos
}

// WriteTo writes the compiled module to the specified output stream.
func (prog *Program) Write(out io.Writer) error { return prog.compiled.Write(out) }

// ExecFile parses, resolves, and executes a Skylark file in the
// specified global environment, which may be modified during execution.
//
// Thread is the state associated with the Skylark thread.
//
// The filename and src parameters are as for syntax.Parse:
// filename is the name of the file to execute,
// and the name that appears in error messages;
// src is an optional source of bytes to use
// instead of filename.
//
// predeclared defines the predeclared names specific to this module.
// Execution does not modify this dictionary, though it may mutate
// its values.
//
// If ExecFile fails during evaluation, it returns an *EvalError
// containing a backtrace.
func ExecFile(thread *Thread, filename string, src interface{}, predeclared StringDict) (StringDict, error) {
	// Parse, resolve, and compile a Skylark source file.
	_, mod, err := SourceProgram(filename, src, predeclared.Has)
	if err != nil {
		return nil, err
	}

	g, err := mod.Init(thread, predeclared)
	g.Freeze()
	return g, err
}

// SourceProgram produces a new program by parsing, resolving,
// and compiling a Skylark source file.
// On success, it returns the parsed file and the compiled program.
// The filename and src parameters are as for syntax.Parse.
//
// The isPredeclared predicate reports whether a name is
// a pre-declared identifier of the current module.
// Its typical value is predeclared.Has,
// where predeclared is a StringDict of pre-declared values.
func SourceProgram(filename string, src interface{}, isPredeclared func(string) bool) (*syntax.File, *Program, error) {
	f, err := syntax.Parse(filename, src, 0)
	if err != nil {
		return nil, nil, err
	}

	if err := resolve.File(f, isPredeclared, Universe.Has); err != nil {
		return f, nil, err
	}

	compiled := compile.File(f.Stmts, f.Locals, f.Globals)

	return f, &Program{compiled}, nil
}

// CompiledProgram produces a new program from the representation
// of a compiled program previously saved by Program.Write.
func CompiledProgram(in io.Reader) (*Program, error) {
	prog, err := compile.ReadProgram(in)
	if err != nil {
		return nil, err
	}
	return &Program{prog}, nil
}

// Init creates a set of global variables for the program,
// executes the toplevel code of the specified program,
// and returns a new, unfrozen dictionary of the globals.
func (prog *Program) Init(thread *Thread, predeclared StringDict) (StringDict, error) {
	toplevel := makeToplevelFunction(prog.compiled.Toplevel, predeclared)

	_, err := toplevel.Call(thread, nil, nil)

	// Convert the global environment to a map and freeze it.
	// We return a (partial) map even in case of error.
	return toplevel.Globals(), err
}

func makeToplevelFunction(funcode *compile.Funcode, predeclared StringDict) *Function {
	// Create the Skylark value denoted by each program constant c.
	constants := make([]Value, len(funcode.Prog.Constants))
	for i, c := range funcode.Prog.Constants {
		var v Value
		switch c := c.(type) {
		case int64:
			v = MakeInt64(c)
		case *big.Int:
			v = Int{c}
		case string:
			v = String(c)
		case float64:
			v = Float(c)
		default:
			log.Fatalf("unexpected constant %T: %v", c, c)
		}
		constants[i] = v
	}

	return &Function{
		funcode:     funcode,
		predeclared: predeclared,
		globals:     make([]Value, len(funcode.Prog.Globals)),
		constants:   constants,
	}
}

// Eval parses, resolves, and evaluates an expression within the
// specified (predeclared) environment.
//
// Evaluation cannot mutate the environment dictionary itself,
// though it may modify variables reachable from the dictionary.
//
// The filename and src parameters are as for syntax.Parse.
//
// If Eval fails during evaluation, it returns an *EvalError
// containing a backtrace.
func Eval(thread *Thread, filename string, src interface{}, env StringDict) (Value, error) {
	expr, err := syntax.ParseExpr(filename, src, 0)
	if err != nil {
		return nil, err
	}

	locals, err := resolve.Expr(expr, env.Has, Universe.Has)
	if err != nil {
		return nil, err
	}

	fn := makeToplevelFunction(compile.Expr(expr, locals), env)

	return fn.Call(thread, nil, nil)
}

// The following functions are primitive operations of the byte code interpreter.

// list += iterable
func listExtend(x *List, y Iterable) {
	if ylist, ok := y.(*List); ok {
		// fast path: list += list
		x.elems = append(x.elems, ylist.elems...)
	} else {
		iter := y.Iterate()
		defer iter.Done()
		var z Value
		for iter.Next(&z) {
			x.elems = append(x.elems, z)
		}
	}
}

// getAttr implements x.dot.
func getAttr(fr *Frame, x Value, name string) (Value, error) {
	// field or method?
	if x, ok := x.(HasAttrs); ok {
		if v, err := x.Attr(name); v != nil || err != nil {
			return v, err
		}
	}

	return nil, fmt.Errorf("%s has no .%s field or method", x.Type(), name)
}

// setField implements x.name = y.
func setField(fr *Frame, x Value, name string, y Value) error {
	if x, ok := x.(HasSetField); ok {
		err := x.SetField(name, y)
		return err
	}
	return fmt.Errorf("can't assign to .%s field of %s", name, x.Type())
}

// getIndex implements x[y].
func getIndex(fr *Frame, x, y Value) (Value, error) {
	switch x := x.(type) {
	case Mapping: // dict
		z, found, err := x.Get(y)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("key %v not in %s", y, x.Type())
		}
		return z, nil

	case Indexable: // string, list, tuple
		n := x.Len()
		i, err := AsInt32(y)
		if err != nil {
			return nil, fmt.Errorf("%s index: %s", x.Type(), err)
		}
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return nil, fmt.Errorf("%s index %d out of range [0:%d]",
				x.Type(), i, n)
		}
		return x.Index(i), nil
	}
	return nil, fmt.Errorf("unhandled index operation %s[%s]", x.Type(), y.Type())
}

// setIndex implements x[y] = z.
func setIndex(fr *Frame, x, y, z Value) error {
	switch x := x.(type) {
	case *Dict:
		if err := x.Set(y, z); err != nil {
			return err
		}

	case HasSetIndex:
		i, err := AsInt32(y)
		if err != nil {
			return err
		}
		if i < 0 {
			i += x.Len()
		}
		if i < 0 || i >= x.Len() {
			return fmt.Errorf("%s index %d out of range [0:%d]", x.Type(), i, x.Len())
		}
		return x.SetIndex(i, z)

	default:
		return fmt.Errorf("%s value does not support item assignment", x.Type())
	}
	return nil
}

// Unary applies a unary operator (+, -, not) to its operand.
func Unary(op syntax.Token, x Value) (Value, error) {
	switch op {
	case syntax.MINUS:
		switch x := x.(type) {
		case Int:
			return zero.Sub(x), nil
		case Float:
			return -x, nil
		}
	case syntax.PLUS:
		switch x.(type) {
		case Int, Float:
			return x, nil
		}
	case syntax.NOT:
		return !x.Truth(), nil
	}
	return nil, fmt.Errorf("unknown unary op: %s %s", op, x.Type())
}

// Binary applies a strict binary operator (not AND or OR) to its operands.
// For equality tests or ordered comparisons, use Compare instead.
func Binary(op syntax.Token, x, y Value) (Value, error) {
	switch op {
	case syntax.PLUS:
		switch x := x.(type) {
		case String:
			if y, ok := y.(String); ok {
				return x + y, nil
			}
		case Int:
			switch y := y.(type) {
			case Int:
				return x.Add(y), nil
			case Float:
				return x.Float() + y, nil
			}
		case Float:
			switch y := y.(type) {
			case Float:
				return x + y, nil
			case Int:
				return x + y.Float(), nil
			}
		case *List:
			if y, ok := y.(*List); ok {
				z := make([]Value, 0, x.Len()+y.Len())
				z = append(z, x.elems...)
				z = append(z, y.elems...)
				return NewList(z), nil
			}
		case Tuple:
			if y, ok := y.(Tuple); ok {
				z := make(Tuple, 0, len(x)+len(y))
				z = append(z, x...)
				z = append(z, y...)
				return z, nil
			}
		case *Dict:
			// Python doesn't have dict+dict, and I can't find
			// it documented for Skylark.  But it is used; see:
			//   tools/build_defs/haskell/def.bzl:448
			// TODO(adonovan): clarify spec; see b/36360157.
			if y, ok := y.(*Dict); ok {
				z := new(Dict)
				for _, item := range x.Items() {
					z.Set(item[0], item[1])
				}
				for _, item := range y.Items() {
					z.Set(item[0], item[1])
				}
				return z, nil
			}
		}

	case syntax.MINUS:
		switch x := x.(type) {
		case Int:
			switch y := y.(type) {
			case Int:
				return x.Sub(y), nil
			case Float:
				return x.Float() - y, nil
			}
		case Float:
			switch y := y.(type) {
			case Float:
				return x - y, nil
			case Int:
				return x - y.Float(), nil
			}
		}

	case syntax.STAR:
		switch x := x.(type) {
		case Int:
			switch y := y.(type) {
			case Int:
				return x.Mul(y), nil
			case Float:
				return x.Float() * y, nil
			case String:
				if i, err := AsInt32(x); err == nil {
					if i < 1 {
						return String(""), nil
					}
					return String(strings.Repeat(string(y), i)), nil
				}
			case *List:
				if i, err := AsInt32(x); err == nil {
					return NewList(repeat(y.elems, i)), nil
				}
			case Tuple:
				if i, err := AsInt32(x); err == nil {
					return Tuple(repeat([]Value(y), i)), nil
				}
			}
		case Float:
			switch y := y.(type) {
			case Float:
				return x * y, nil
			case Int:
				return x * y.Float(), nil
			}
		case String:
			if y, ok := y.(Int); ok {
				if i, err := AsInt32(y); err == nil {
					if i < 1 {
						return String(""), nil
					}
					return String(strings.Repeat(string(x), i)), nil
				}
			}
		case *List:
			if y, ok := y.(Int); ok {
				if i, err := AsInt32(y); err == nil {
					return NewList(repeat(x.elems, i)), nil
				}
			}
		case Tuple:
			if y, ok := y.(Int); ok {
				if i, err := AsInt32(y); err == nil {
					return Tuple(repeat([]Value(x), i)), nil
				}
			}

		}

	case syntax.SLASH:
		switch x := x.(type) {
		case Int:
			switch y := y.(type) {
			case Int:
				yf := y.Float()
				if yf == 0.0 {
					return nil, fmt.Errorf("real division by zero")
				}
				return x.Float() / yf, nil
			case Float:
				if y == 0.0 {
					return nil, fmt.Errorf("real division by zero")
				}
				return x.Float() / y, nil
			}
		case Float:
			switch y := y.(type) {
			case Float:
				if y == 0.0 {
					return nil, fmt.Errorf("real division by zero")
				}
				return x / y, nil
			case Int:
				yf := y.Float()
				if yf == 0.0 {
					return nil, fmt.Errorf("real division by zero")
				}
				return x / yf, nil
			}
		}

	case syntax.SLASHSLASH:
		switch x := x.(type) {
		case Int:
			switch y := y.(type) {
			case Int:
				if y.Sign() == 0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return x.Div(y), nil
			case Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor((x.Float() / y)), nil
			}
		case Float:
			switch y := y.(type) {
			case Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor(x / y), nil
			case Int:
				yf := y.Float()
				if yf == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor(x / yf), nil
			}
		}

	case syntax.PERCENT:
		switch x := x.(type) {
		case Int:
			switch y := y.(type) {
			case Int:
				if y.Sign() == 0 {
					return nil, fmt.Errorf("integer modulo by zero")
				}
				return x.Mod(y), nil
			case Float:
				if y == 0 {
					return nil, fmt.Errorf("float modulo by zero")
				}
				return x.Float().Mod(y), nil
			}
		case Float:
			switch y := y.(type) {
			case Float:
				if y == 0.0 {
					return nil, fmt.Errorf("float modulo by zero")
				}
				return Float(math.Mod(float64(x), float64(y))), nil
			case Int:
				if y.Sign() == 0 {
					return nil, fmt.Errorf("float modulo by zero")
				}
				return x.Mod(y.Float()), nil
			}
		case String:
			return interpolate(string(x), y)
		}

	case syntax.NOT_IN:
		z, err := Binary(syntax.IN, x, y)
		if err != nil {
			return nil, err
		}
		return !z.Truth(), nil

	case syntax.IN:
		switch y := y.(type) {
		case *List:
			for _, elem := range y.elems {
				if eq, err := Equal(elem, x); err != nil {
					return nil, err
				} else if eq {
					return True, nil
				}
			}
			return False, nil
		case Tuple:
			for _, elem := range y {
				if eq, err := Equal(elem, x); err != nil {
					return nil, err
				} else if eq {
					return True, nil
				}
			}
			return False, nil
		case Mapping: // e.g. dict
			// Ignore error from Get as we cannot distinguish true
			// errors (value cycle, type error) from "key not found".
			_, found, _ := y.Get(x)
			return Bool(found), nil
		case *Set:
			ok, err := y.Has(x)
			return Bool(ok), err
		case String:
			needle, ok := x.(String)
			if !ok {
				return nil, fmt.Errorf("'in <string>' requires string as left operand, not %s", x.Type())
			}
			return Bool(strings.Contains(string(y), string(needle))), nil
		case rangeValue:
			i, err := NumberToInt(x)
			if err != nil {
				return nil, fmt.Errorf("'in <range>' requires integer as left operand, not %s", x.Type())
			}
			return Bool(y.contains(i)), nil
		}

	case syntax.PIPE:
		switch x := x.(type) {
		case Int:
			if y, ok := y.(Int); ok {
				return x.Or(y), nil
			}
		case *Set: // union
			if y, ok := y.(*Set); ok {
				iter := Iterate(y)
				defer iter.Done()
				return x.Union(iter)
			}
		}

	case syntax.AMP:
		switch x := x.(type) {
		case Int:
			if y, ok := y.(Int); ok {
				return x.And(y), nil
			}
		case *Set: // intersection
			if y, ok := y.(*Set); ok {
				set := new(Set)
				if x.Len() > y.Len() {
					x, y = y, x // opt: range over smaller set
				}
				for _, xelem := range x.elems() {
					// Has, Insert cannot fail here.
					if found, _ := y.Has(xelem); found {
						set.Insert(xelem)
					}
				}
				return set, nil
			}
		}

	default:
		// unknown operator
		goto unknown
	}

	// user-defined types
	if x, ok := x.(HasBinary); ok {
		z, err := x.Binary(op, y, Left)
		if z != nil || err != nil {
			return z, err
		}
	}
	if y, ok := y.(HasBinary); ok {
		z, err := y.Binary(op, x, Right)
		if z != nil || err != nil {
			return z, err
		}
	}

	// unsupported operand types
unknown:
	return nil, fmt.Errorf("unknown binary op: %s %s %s", x.Type(), op, y.Type())
}

func repeat(elems []Value, n int) (res []Value) {
	if n > 0 {
		res = make([]Value, 0, len(elems)*n)
		for i := 0; i < n; i++ {
			res = append(res, elems...)
		}
	}
	return res
}

// Call calls the function fn with the specified positional and keyword arguments.
func Call(thread *Thread, fn Value, args Tuple, kwargs []Tuple) (Value, error) {
	c, ok := fn.(Callable)
	if !ok {
		return nil, fmt.Errorf("invalid call of non-function (%s)", fn.Type())
	}
	res, err := c.Call(thread, args, kwargs)
	// Sanity check: nil is not a valid Skylark value.
	if err == nil && res == nil {
		return nil, fmt.Errorf("internal error: nil (not None) returned from %s", fn)
	}
	return res, err
}

func slice(x, lo, hi, step_ Value) (Value, error) {
	sliceable, ok := x.(Sliceable)
	if !ok {
		return nil, fmt.Errorf("invalid slice operand %s", x.Type())
	}

	n := sliceable.Len()
	step := 1
	if step_ != None {
		var err error
		step, err = AsInt32(step_)
		if err != nil {
			return nil, fmt.Errorf("got %s for slice step, want int", step_.Type())
		}
		if step == 0 {
			return nil, fmt.Errorf("zero is not a valid slice step")
		}
	}

	// TODO(adonovan): opt: preallocate result array.

	var start, end int
	if step > 0 {
		// positive stride
		// default indices are [0:n].
		var err error
		start, end, err = indices(lo, hi, n)
		if err != nil {
			return nil, err
		}

		if end < start {
			end = start // => empty result
		}
	} else {
		// negative stride
		// default indices are effectively [n-1:-1], though to
		// get this effect using explicit indices requires
		// [n-1:-1-n:-1] because of the treatment of -ve values.
		start = n - 1
		if err := asIndex(lo, n, &start); err != nil {
			return nil, fmt.Errorf("invalid start index: %s", err)
		}
		if start >= n {
			start = n - 1
		}

		end = -1
		if err := asIndex(hi, n, &end); err != nil {
			return nil, fmt.Errorf("invalid end index: %s", err)
		}
		if end < -1 {
			end = -1
		}

		if start < end {
			start = end // => empty result
		}
	}

	return sliceable.Slice(start, end, step), nil
}

// From Hacker's Delight, section 2.8.
func signum(x int) int { return int(uint64(int64(x)>>63) | (uint64(-x) >> 63)) }

// indices converts start_ and end_ to indices in the range [0:len].
// The start index defaults to 0 and the end index defaults to len.
// An index -len < i < 0 is treated like i+len.
// All other indices outside the range are clamped to the nearest value in the range.
// Beware: start may be greater than end.
// This function is suitable only for slices with positive strides.
func indices(start_, end_ Value, len int) (start, end int, err error) {
	start = 0
	if err := asIndex(start_, len, &start); err != nil {
		return 0, 0, fmt.Errorf("invalid start index: %s", err)
	}
	// Clamp to [0:len].
	if start < 0 {
		start = 0
	} else if start > len {
		start = len
	}

	end = len
	if err := asIndex(end_, len, &end); err != nil {
		return 0, 0, fmt.Errorf("invalid end index: %s", err)
	}
	// Clamp to [0:len].
	if end < 0 {
		end = 0
	} else if end > len {
		end = len
	}

	return start, end, nil
}

// asIndex sets *result to the integer value of v, adding len to it
// if it is negative.  If v is nil or None, *result is unchanged.
func asIndex(v Value, len int, result *int) error {
	if v != nil && v != None {
		var err error
		*result, err = AsInt32(v)
		if err != nil {
			return fmt.Errorf("got %s, want int", v.Type())
		}
		if *result < 0 {
			*result += len
		}
	}
	return nil
}

// setArgs sets the values of the formal parameters of function fn in
// based on the actual parameter values in args and kwargs.
func setArgs(locals []Value, fn *Function, args Tuple, kwargs []Tuple) error {
	cond := func(x bool, y, z interface{}) interface{} {
		if x {
			return y
		}
		return z
	}

	// nparams is the number of ordinary parameters (sans * or **).
	nparams := fn.NumParams()
	if fn.HasVarargs() {
		nparams--
	}
	if fn.HasKwargs() {
		nparams--
	}

	// This is the algorithm from PyEval_EvalCodeEx.
	var kwdict *Dict
	n := len(args)
	if nparams > 0 || fn.HasVarargs() || fn.HasKwargs() {
		if fn.HasKwargs() {
			kwdict = new(Dict)
			locals[fn.NumParams()-1] = kwdict
		}

		// too many args?
		if len(args) > nparams {
			if !fn.HasVarargs() {
				return fmt.Errorf("function %s takes %s %d argument%s (%d given)",
					fn.Name(),
					cond(len(fn.defaults) > 0, "at most", "exactly"),
					nparams,
					cond(nparams == 1, "", "s"),
					len(args)+len(kwargs))
			}
			n = nparams
		}

		// set of defined (regular) parameters
		var defined intset
		defined.init(nparams)

		// ordinary parameters
		for i := 0; i < n; i++ {
			locals[i] = args[i]
			defined.set(i)
		}

		// variadic arguments
		if fn.HasVarargs() {
			tuple := make(Tuple, len(args)-n)
			for i := n; i < len(args); i++ {
				tuple[i-n] = args[i]
			}
			locals[nparams] = tuple
		}

		// keyword arguments
		paramIdents := fn.funcode.Locals[:nparams]
		for _, pair := range kwargs {
			k, v := pair[0].(String), pair[1]
			if i := findParam(paramIdents, string(k)); i >= 0 {
				if defined.set(i) {
					return fmt.Errorf("function %s got multiple values for keyword argument %s", fn.Name(), k)
				}
				locals[i] = v
				continue
			}
			if kwdict == nil {
				return fmt.Errorf("function %s got an unexpected keyword argument %s", fn.Name(), k)
			}
			kwdict.Set(k, v)
		}

		// default values
		if len(args) < nparams {
			m := nparams - len(fn.defaults) // first default

			// report errors for missing non-optional arguments
			i := len(args)
			for ; i < m; i++ {
				if !defined.get(i) {
					return fmt.Errorf("function %s takes %s %d argument%s (%d given)",
						fn.Name(),
						cond(fn.HasVarargs() || len(fn.defaults) > 0, "at least", "exactly"),
						m,
						cond(m == 1, "", "s"),
						defined.len())
				}
			}

			// set default values
			for ; i < nparams; i++ {
				if !defined.get(i) {
					locals[i] = fn.defaults[i-m]
				}
			}
		}
	} else if nactual := len(args) + len(kwargs); nactual > 0 {
		return fmt.Errorf("function %s takes no arguments (%d given)", fn.Name(), nactual)
	}
	return nil
}

func findParam(params []compile.Ident, name string) int {
	for i, param := range params {
		if param.Name == name {
			return i
		}
	}
	return -1
}

type intset struct {
	small uint64       // bitset, used if n < 64
	large map[int]bool //    set, used if n >= 64
}

func (is *intset) init(n int) {
	if n >= 64 {
		is.large = make(map[int]bool)
	}
}

func (is *intset) set(i int) (prev bool) {
	if is.large == nil {
		prev = is.small&(1<<uint(i)) != 0
		is.small |= 1 << uint(i)
	} else {
		prev = is.large[i]
		is.large[i] = true
	}
	return
}

func (is *intset) get(i int) bool {
	if is.large == nil {
		return is.small&(1<<uint(i)) != 0
	}
	return is.large[i]
}

func (is *intset) len() int {
	if is.large == nil {
		// Suboptimal, but used only for error reporting.
		len := 0
		for i := 0; i < 64; i++ {
			if is.small&(1<<uint(i)) != 0 {
				len++
			}
		}
		return len
	}
	return len(is.large)
}

// https://github.com/google/skylark/blob/master/doc/spec.md#string-interpolation
func interpolate(format string, x Value) (Value, error) {
	var buf bytes.Buffer
	path := make([]Value, 0, 4)
	index := 0
	for {
		i := strings.IndexByte(format, '%')
		if i < 0 {
			buf.WriteString(format)
			break
		}
		buf.WriteString(format[:i])
		format = format[i+1:]

		if format != "" && format[0] == '%' {
			buf.WriteByte('%')
			format = format[1:]
			continue
		}

		var arg Value
		if format != "" && format[0] == '(' {
			// keyword argument: %(name)s.
			format = format[1:]
			j := strings.IndexByte(format, ')')
			if j < 0 {
				return nil, fmt.Errorf("incomplete format key")
			}
			key := format[:j]
			if dict, ok := x.(Mapping); !ok {
				return nil, fmt.Errorf("format requires a mapping")
			} else if v, found, _ := dict.Get(String(key)); found {
				arg = v
			} else {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			format = format[j+1:]
		} else {
			// positional argument: %s.
			if tuple, ok := x.(Tuple); ok {
				if index >= len(tuple) {
					return nil, fmt.Errorf("not enough arguments for format string")
				}
				arg = tuple[index]
			} else if index > 0 {
				return nil, fmt.Errorf("not enough arguments for format string")
			} else {
				arg = x
			}
		}

		// NOTE: Skylark does not support any of these optional Python features:
		// - optional conversion flags: [#0- +], etc.
		// - optional minimum field width (number or *).
		// - optional precision (.123 or *)
		// - optional length modifier

		// conversion type
		if format == "" {
			return nil, fmt.Errorf("incomplete format")
		}
		switch c := format[0]; c {
		case 's', 'r':
			if str, ok := AsString(arg); ok && c == 's' {
				buf.WriteString(str)
			} else {
				writeValue(&buf, arg, path)
			}
		case 'd', 'i', 'o', 'x', 'X':
			i, err := NumberToInt(arg)
			if err != nil {
				return nil, fmt.Errorf("%%%c format requires integer: %v", c, err)
			}
			switch c {
			case 'd', 'i':
				buf.WriteString(i.bigint.Text(10))
			case 'o':
				buf.WriteString(i.bigint.Text(8))
			case 'x':
				buf.WriteString(i.bigint.Text(16))
			case 'X':
				buf.WriteString(strings.ToUpper(i.bigint.Text(16)))
			}
		case 'e', 'f', 'g', 'E', 'F', 'G':
			f, ok := AsFloat(arg)
			if !ok {
				return nil, fmt.Errorf("%%%c format requires float, not %s", c, arg.Type())
			}
			switch c {
			case 'e':
				fmt.Fprintf(&buf, "%e", f)
			case 'f':
				fmt.Fprintf(&buf, "%f", f)
			case 'g':
				fmt.Fprintf(&buf, "%g", f)
			case 'E':
				fmt.Fprintf(&buf, "%E", f)
			case 'F':
				fmt.Fprintf(&buf, "%F", f)
			case 'G':
				fmt.Fprintf(&buf, "%G", f)
			}
		case 'c':
			switch arg := arg.(type) {
			case Int:
				// chr(int)
				r, err := AsInt32(arg)
				if err != nil || r < 0 || r > unicode.MaxRune {
					return nil, fmt.Errorf("%%c format requires a valid Unicode code point, got %s", arg)
				}
				buf.WriteRune(rune(r))
			case String:
				r, size := utf8.DecodeRuneInString(string(arg))
				if size != len(arg) {
					return nil, fmt.Errorf("%%c format requires a single-character string")
				}
				buf.WriteRune(r)
			default:
				return nil, fmt.Errorf("%%c format requires int or single-character string, not %s", arg.Type())
			}
		case '%':
			buf.WriteByte('%')
		default:
			return nil, fmt.Errorf("unknown conversion %%%c", c)
		}
		format = format[1:]
		index++
	}

	if tuple, ok := x.(Tuple); ok && index < len(tuple) {
		return nil, fmt.Errorf("too many arguments for format string")
	}

	return String(buf.String()), nil
}
