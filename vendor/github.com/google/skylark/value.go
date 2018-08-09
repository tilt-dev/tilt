// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package skylark provides a Skylark interpreter.
//
// Skylark values are represented by the Value interface.
// The following built-in Value types are known to the evaluator:
//
//      NoneType        -- NoneType
//      Bool            -- bool
//      Int             -- int
//      Float           -- float
//      String          -- string
//      *List           -- list
//      Tuple           -- tuple
//      *Dict           -- dict
//      *Set            -- set
//      *Function       -- function (implemented in Skylark)
//      *Builtin        -- builtin_function_or_method (function or method implemented in Go)
//
// Client applications may define new data types that satisfy at least
// the Value interface.  Such types may provide additional operations by
// implementing any of these optional interfaces:
//
//      Callable        -- value is callable like a function
//      Comparable      -- value defines its own comparison operations
//      Iterable        -- value is iterable using 'for' loops
//      Sequence        -- value is iterable sequence of known length
//      Indexable       -- value is sequence with efficient random access
//      HasBinary       -- value defines binary operations such as * and +
//      HasAttrs        -- value has readable fields or methods x.f
//      HasSetField     -- value has settable fields x.f
//      HasSetIndex     -- value supports element update using x[i]=y
//
// Client applications may also define domain-specific functions in Go
// and make them available to Skylark programs.  Use NewBuiltin to
// construct a built-in value that wraps a Go function.  The
// implementation of the Go function may use UnpackArgs to make sense of
// the positional and keyword arguments provided by the caller.
//
// Skylark's None value is not equal to Go's nil, but nil may be
// assigned to a Skylark Value.  Be careful to avoid allowing Go nil
// values to leak into Skylark data structures.
//
// The Compare operation requires two arguments of the same
// type, but this constraint cannot be expressed in Go's type system.
// (This is the classic "binary method problem".)
// So, each Value type's CompareSameType method is a partial function
// that compares a value only against others of the same type.
// Use the package's standalone Compare (or Equal) function to compare
// an arbitrary pair of values.
//
// To parse and evaluate a Skylark source file, use ExecFile.  The Eval
// function evaluates a single expression.  All evaluator functions
// require a Thread parameter which defines the "thread-local storage"
// of a Skylark thread and may be used to plumb application state
// through Sklyark code and into callbacks.  When evaluation fails it
// returns an EvalError from which the application may obtain a
// backtrace of active Skylark calls.
//
package skylark

// This file defines the data types of Skylark and their basic operations.

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/skylark/internal/compile"
	"github.com/google/skylark/syntax"
)

// Value is a value in the Skylark interpreter.
type Value interface {
	// String returns the string representation of the value.
	// Skylark string values are quoted as if by Python's repr.
	String() string

	// Type returns a short string describing the value's type.
	Type() string

	// Freeze causes the value, and all values transitively
	// reachable from it through collections and closures, to be
	// marked as frozen.  All subsequent mutations to the data
	// structure through this API will fail dynamically, making the
	// data structure immutable and safe for publishing to other
	// Skylark interpreters running concurrently.
	Freeze()

	// Truth returns the truth value of an object.
	Truth() Bool

	// Hash returns a function of x such that Equals(x, y) => Hash(x) == Hash(y).
	// Hash may fail if the value's type is not hashable, or if the value
	// contains a non-hashable value.
	Hash() (uint32, error)
}

// A Comparable is a value that defines its own equivalence relation and
// perhaps ordered comparisons.
type Comparable interface {
	Value
	// CompareSameType compares one value to another of the same Type().
	// The comparison operation must be one of EQL, NEQ, LT, LE, GT, or GE.
	// CompareSameType returns an error if an ordered comparison was
	// requested for a type that does not support it.
	//
	// Implementations that recursively compare subcomponents of
	// the value should use the CompareDepth function, not Compare, to
	// avoid infinite recursion on cyclic structures.
	//
	// The depth parameter is used to bound comparisons of cyclic
	// data structures.  Implementations should decrement depth
	// before calling CompareDepth and should return an error if depth
	// < 1.
	//
	// Client code should not call this method.  Instead, use the
	// standalone Compare or Equals functions, which are defined for
	// all pairs of operands.
	CompareSameType(op syntax.Token, y Value, depth int) (bool, error)
}

var (
	_ Comparable = None
	_ Comparable = Int{}
	_ Comparable = False
	_ Comparable = Float(0)
	_ Comparable = String("")
	_ Comparable = (*Dict)(nil)
	_ Comparable = (*List)(nil)
	_ Comparable = Tuple(nil)
	_ Comparable = (*Set)(nil)
)

// A Callable value f may be the operand of a function call, f(x).
type Callable interface {
	Value
	Name() string
	Call(thread *Thread, args Tuple, kwargs []Tuple) (Value, error)
}

var (
	_ Callable = (*Builtin)(nil)
	_ Callable = (*Function)(nil)
)

// An Iterable abstracts a sequence of values.
// An iterable value may be iterated over by a 'for' loop or used where
// any other Skylark iterable is allowed.  Unlike a Sequence, the length
// of an Iterable is not necessarily known in advance of iteration.
type Iterable interface {
	Value
	Iterate() Iterator // must be followed by call to Iterator.Done
}

// A Sequence is a sequence of values of known length.
type Sequence interface {
	Iterable
	Len() int
}

var (
	_ Sequence = (*Dict)(nil)
	_ Sequence = (*Set)(nil)
)

// An Indexable is a sequence of known length that supports efficient random access.
// It is not necessarily iterable.
type Indexable interface {
	Value
	Index(i int) Value // requires 0 <= i < Len()
	Len() int
}

// A Sliceable is a sequence that can be cut into pieces with the slice operator (x[i:j:step]).
//
// All native indexable objects are sliceable.
// This is a separate interface for backwards-compatibility.
type Sliceable interface {
	Indexable
	// For positive strides (step > 0), 0 <= start <= end <= n.
	// For negative strides (step < 0), -1 <= end <= start < n.
	// The caller must ensure that the start and end indices are valid.
	Slice(start, end, step int) Value
}

// A HasSetIndex is an Indexable value whose elements may be assigned (x[i] = y).
//
// The implementation should not add Len to a negative index as the
// evaluator does this before the call.
type HasSetIndex interface {
	Indexable
	SetIndex(index int, v Value) error
}

var (
	_ HasSetIndex = (*List)(nil)
	_ Indexable   = Tuple(nil)
	_ Indexable   = String("")
	_ Sliceable   = Tuple(nil)
	_ Sliceable   = String("")
	_ Sliceable   = (*List)(nil)
)

// An Iterator provides a sequence of values to the caller.
//
// The caller must call Done when the iterator is no longer needed.
// Operations that modify a sequence will fail if it has active iterators.
//
// Example usage:
//
// 	iter := iterable.Iterator()
//	defer iter.Done()
//	var x Value
//	for iter.Next(&x) {
//		...
//	}
//
type Iterator interface {
	// If the iterator is exhausted, Next returns false.
	// Otherwise it sets *p to the current element of the sequence,
	// advances the iterator, and returns true.
	Next(p *Value) bool
	Done()
}

// An Mapping is a mapping from keys to values, such as a dictionary.
type Mapping interface {
	Value
	// Get returns the value corresponding to the specified key,
	// or !found if the mapping does not contain the key.
	//
	// Get also defines the behavior of "v in mapping".
	// The 'in' operator reports the 'found' component, ignoring errors.
	Get(Value) (v Value, found bool, err error)
}

var _ Mapping = (*Dict)(nil)

// A HasBinary value may be used as either operand of these binary operators:
//     +   -   *   /   %   in   not in   |   &
// The Side argument indicates whether the receiver is the left or right operand.
//
// An implementation may decline to handle an operation by returning (nil, nil).
// For this reason, clients should always call the standalone Binary(op, x, y)
// function rather than calling the method directly.
type HasBinary interface {
	Value
	Binary(op syntax.Token, y Value, side Side) (Value, error)
}

type Side bool

const (
	Left  Side = false
	Right Side = true
)

// A HasAttrs value has fields or methods that may be read by a dot expression (y = x.f).
// Attribute names may be listed using the built-in 'dir' function.
//
// For implementation convenience, a result of (nil, nil) from Attr is
// interpreted as a "no such field or method" error. Implementations are
// free to return a more precise error.
type HasAttrs interface {
	Value
	Attr(name string) (Value, error) // returns (nil, nil) if attribute not present
	AttrNames() []string             // callers must not modify the result.
}

var (
	_ HasAttrs = String("")
	_ HasAttrs = new(List)
	_ HasAttrs = new(Dict)
	_ HasAttrs = new(Set)
)

// A HasSetField value has fields that may be written by a dot expression (x.f = y).
type HasSetField interface {
	HasAttrs
	SetField(name string, val Value) error
}

// NoneType is the type of None.  Its only legal value is None.
// (We represent it as a number, not struct{}, so that None may be constant.)
type NoneType byte

const None = NoneType(0)

func (NoneType) String() string        { return "None" }
func (NoneType) Type() string          { return "NoneType" }
func (NoneType) Freeze()               {} // immutable
func (NoneType) Truth() Bool           { return False }
func (NoneType) Hash() (uint32, error) { return 0, nil }
func (NoneType) CompareSameType(op syntax.Token, y Value, depth int) (bool, error) {
	return threeway(op, 0), nil
}

// Bool is the type of a Skylark bool.
type Bool bool

const (
	False Bool = false
	True  Bool = true
)

func (b Bool) String() string {
	if b {
		return "True"
	} else {
		return "False"
	}
}
func (b Bool) Type() string          { return "bool" }
func (b Bool) Freeze()               {} // immutable
func (b Bool) Truth() Bool           { return b }
func (b Bool) Hash() (uint32, error) { return uint32(b2i(bool(b))), nil }
func (x Bool) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(Bool)
	return threeway(op, b2i(bool(x))-b2i(bool(y))), nil
}

// Float is the type of a Skylark float.
type Float float64

func (f Float) String() string { return strconv.FormatFloat(float64(f), 'g', 6, 64) }
func (f Float) Type() string   { return "float" }
func (f Float) Freeze()        {} // immutable
func (f Float) Truth() Bool    { return f != 0.0 }
func (f Float) Hash() (uint32, error) {
	// Equal float and int values must yield the same hash.
	// TODO(adonovan): opt: if f is non-integral, and thus not equal
	// to any Int, we can avoid the Int conversion and use a cheaper hash.
	if isFinite(float64(f)) {
		return finiteFloatToInt(f).Hash()
	}
	return 1618033, nil // NaN, +/-Inf
}

func floor(f Float) Float { return Float(math.Floor(float64(f))) }

// isFinite reports whether f represents a finite rational value.
// It is equivalent to !math.IsNan(f) && !math.IsInf(f, 0).
func isFinite(f float64) bool {
	return math.Abs(f) <= math.MaxFloat64
}

func (x Float) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(Float)
	switch op {
	case syntax.EQL:
		return x == y, nil
	case syntax.NEQ:
		return x != y, nil
	case syntax.LE:
		return x <= y, nil
	case syntax.LT:
		return x < y, nil
	case syntax.GE:
		return x >= y, nil
	case syntax.GT:
		return x > y, nil
	}
	panic(op)
}

func (f Float) rational() *big.Rat { return new(big.Rat).SetFloat64(float64(f)) }

// AsFloat returns the float64 value closest to x.
// The f result is undefined if x is not a float or int.
func AsFloat(x Value) (f float64, ok bool) {
	switch x := x.(type) {
	case Float:
		return float64(x), true
	case Int:
		return float64(x.Float()), true
	}
	return 0, false
}

func (x Float) Mod(y Float) Float { return Float(math.Mod(float64(x), float64(y))) }

// String is the type of a Skylark string.
//
// A String encapsulates an an immutable sequence of bytes,
// but strings are not directly iterable. Instead, iterate
// over the result of calling one of these four methods:
// codepoints, codepoint_ords, elems, elem_ords.
type String string

func (s String) String() string        { return strconv.Quote(string(s)) }
func (s String) Type() string          { return "string" }
func (s String) Freeze()               {} // immutable
func (s String) Truth() Bool           { return len(s) > 0 }
func (s String) Hash() (uint32, error) { return hashString(string(s)), nil }
func (s String) Len() int              { return len(s) } // bytes
func (s String) Index(i int) Value     { return s[i : i+1] }

func (s String) Slice(start, end, step int) Value {
	if step == 1 {
		return String(s[start:end])
	}

	sign := signum(step)
	var str []byte
	for i := start; signum(end-i) == sign; i += step {
		str = append(str, s[i])
	}
	return String(str)
}

func (s String) Attr(name string) (Value, error) { return builtinAttr(s, name, stringMethods) }
func (s String) AttrNames() []string             { return builtinAttrNames(stringMethods) }

func (x String) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(String)
	return threeway(op, strings.Compare(string(x), string(y))), nil
}

func AsString(x Value) (string, bool) { v, ok := x.(String); return string(v), ok }

// A stringIterable is an iterable whose iterator yields a sequence of
// either Unicode code points or elements (bytes),
// either numerically or as successive substrings.
type stringIterable struct {
	s          String
	ords       bool
	codepoints bool
}

var _ Iterable = (*stringIterable)(nil)

func (si stringIterable) String() string {
	var etype string
	if si.codepoints {
		etype = "codepoint"
	} else {
		etype = "elem"
	}
	if si.ords {
		return si.s.String() + "." + etype + "_ords()"
	} else {
		return si.s.String() + "." + etype + "s()"
	}
}
func (si stringIterable) Type() string {
	if si.codepoints {
		return "codepoints"
	} else {
		return "elems"
	}
}
func (si stringIterable) Freeze()               {} // immutable
func (si stringIterable) Truth() Bool           { return True }
func (si stringIterable) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: %s", si.Type()) }
func (si stringIterable) Iterate() Iterator     { return &stringIterator{si, 0} }

type stringIterator struct {
	si stringIterable
	i  int
}

func (it *stringIterator) Next(p *Value) bool {
	s := it.si.s[it.i:]
	if s == "" {
		return false
	}
	if it.si.codepoints {
		r, sz := utf8.DecodeRuneInString(string(s))
		if !it.si.ords {
			*p = s[:sz]
		} else {
			*p = MakeInt(int(r))
		}
		it.i += sz
	} else {
		b := int(s[0])
		if !it.si.ords {
			*p = s[:1]
		} else {
			*p = MakeInt(b)
		}
		it.i += 1
	}
	return true
}

func (*stringIterator) Done() {}

// A Function is a function defined by a Skylark def statement or lambda expression.
// The initialization behavior of a Skylark module is also represented by a Function.
type Function struct {
	funcode  *compile.Funcode
	defaults Tuple
	freevars Tuple

	// These fields are shared by all functions in a module.
	predeclared StringDict
	globals     []Value
	constants   []Value
}

func (fn *Function) Name() string          { return fn.funcode.Name } // "lambda" for anonymous functions
func (fn *Function) Hash() (uint32, error) { return hashString(fn.funcode.Name), nil }
func (fn *Function) Freeze()               { fn.defaults.Freeze(); fn.freevars.Freeze() }
func (fn *Function) String() string        { return toString(fn) }
func (fn *Function) Type() string          { return "function" }
func (fn *Function) Truth() Bool           { return true }

// Globals returns a new, unfrozen StringDict containing all global
// variables so far defined in the function's module.
func (fn *Function) Globals() StringDict {
	m := make(StringDict, len(fn.funcode.Prog.Globals))
	for i, id := range fn.funcode.Prog.Globals {
		if v := fn.globals[i]; v != nil {
			m[id.Name] = v
		}
	}
	return m
}

func (fn *Function) Position() syntax.Position { return fn.funcode.Pos }
func (fn *Function) NumParams() int            { return fn.funcode.NumParams }
func (fn *Function) Param(i int) (string, syntax.Position) {
	id := fn.funcode.Locals[i]
	return id.Name, id.Pos
}
func (fn *Function) HasVarargs() bool { return fn.funcode.HasVarargs }
func (fn *Function) HasKwargs() bool  { return fn.funcode.HasKwargs }

// A Builtin is a function implemented in Go.
type Builtin struct {
	name string
	fn   func(thread *Thread, fn *Builtin, args Tuple, kwargs []Tuple) (Value, error)
	recv Value // for bound methods (e.g. "".startswith)
}

func (b *Builtin) Name() string { return b.name }
func (b *Builtin) Freeze() {
	if b.recv != nil {
		b.recv.Freeze()
	}
}
func (b *Builtin) Hash() (uint32, error) {
	h := hashString(b.name)
	if b.recv != nil {
		h ^= 5521
	}
	return h, nil
}
func (b *Builtin) Receiver() Value { return b.recv }
func (b *Builtin) String() string  { return toString(b) }
func (b *Builtin) Type() string    { return "builtin_function_or_method" }
func (b *Builtin) Call(thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	thread.frame = &Frame{parent: thread.frame, callable: b}
	result, err := b.fn(thread, b, args, kwargs)
	thread.frame = thread.frame.parent
	return result, err
}
func (b *Builtin) Truth() Bool { return true }

// NewBuiltin returns a new 'builtin_function_or_method' value with the specified name
// and implementation.  It compares unequal with all other values.
func NewBuiltin(name string, fn func(thread *Thread, fn *Builtin, args Tuple, kwargs []Tuple) (Value, error)) *Builtin {
	return &Builtin{name: name, fn: fn}
}

// BindReceiver returns a new Builtin value representing a method
// closure, that is, a built-in function bound to a receiver value.
//
// In the example below, the value of f is the string.index
// built-in method bound to the receiver value "abc":
//
//     f = "abc".index; f("a"); f("b")
//
// In the common case, the receiver is bound only during the call,
// but this still results in the creation of a temporary method closure:
//
//     "abc".index("a")
//
func (b *Builtin) BindReceiver(recv Value) *Builtin {
	return &Builtin{name: b.name, fn: b.fn, recv: recv}
}

// A *Dict represents a Skylark dictionary.
type Dict struct {
	ht hashtable
}

func (d *Dict) Clear() error                                    { return d.ht.clear() }
func (d *Dict) Delete(k Value) (v Value, found bool, err error) { return d.ht.delete(k) }
func (d *Dict) Get(k Value) (v Value, found bool, err error)    { return d.ht.lookup(k) }
func (d *Dict) Items() []Tuple                                  { return d.ht.items() }
func (d *Dict) Keys() []Value                                   { return d.ht.keys() }
func (d *Dict) Len() int                                        { return int(d.ht.len) }
func (d *Dict) Iterate() Iterator                               { return d.ht.iterate() }
func (d *Dict) Set(k, v Value) error                            { return d.ht.insert(k, v) }
func (d *Dict) String() string                                  { return toString(d) }
func (d *Dict) Type() string                                    { return "dict" }
func (d *Dict) Freeze()                                         { d.ht.freeze() }
func (d *Dict) Truth() Bool                                     { return d.Len() > 0 }
func (d *Dict) Hash() (uint32, error)                           { return 0, fmt.Errorf("unhashable type: dict") }

func (d *Dict) Attr(name string) (Value, error) { return builtinAttr(d, name, dictMethods) }
func (d *Dict) AttrNames() []string             { return builtinAttrNames(dictMethods) }

func (x *Dict) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*Dict)
	switch op {
	case syntax.EQL:
		ok, err := dictsEqual(x, y, depth)
		return ok, err
	case syntax.NEQ:
		ok, err := dictsEqual(x, y, depth)
		return !ok, err
	default:
		return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
	}
}

func dictsEqual(x, y *Dict, depth int) (bool, error) {
	if x.Len() != y.Len() {
		return false, nil
	}
	for _, xitem := range x.Items() {
		key, xval := xitem[0], xitem[1]

		if yval, found, _ := y.Get(key); !found {
			return false, nil
		} else if eq, err := EqualDepth(xval, yval, depth-1); err != nil {
			return false, err
		} else if !eq {
			return false, nil
		}
	}
	return true, nil
}

// A *List represents a Skylark list value.
type List struct {
	elems     []Value
	frozen    bool
	itercount uint32 // number of active iterators (ignored if frozen)
}

// NewList returns a list containing the specified elements.
// Callers should not subsequently modify elems.
func NewList(elems []Value) *List { return &List{elems: elems} }

func (l *List) Freeze() {
	if !l.frozen {
		l.frozen = true
		for _, elem := range l.elems {
			elem.Freeze()
		}
	}
}

// checkMutable reports an error if the list should not be mutated.
// verb+" list" should describe the operation.
// Structural mutations are not permitted during iteration.
func (l *List) checkMutable(verb string, structural bool) error {
	if l.frozen {
		return fmt.Errorf("cannot %s frozen list", verb)
	}
	if structural && l.itercount > 0 {
		return fmt.Errorf("cannot %s list during iteration", verb)
	}
	return nil
}

func (l *List) String() string        { return toString(l) }
func (l *List) Type() string          { return "list" }
func (l *List) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: list") }
func (l *List) Truth() Bool           { return l.Len() > 0 }
func (l *List) Len() int              { return len(l.elems) }
func (l *List) Index(i int) Value     { return l.elems[i] }

func (l *List) Slice(start, end, step int) Value {
	if step == 1 {
		elems := append([]Value{}, l.elems[start:end]...)
		return NewList(elems)
	}

	sign := signum(step)
	var list []Value
	for i := start; signum(end-i) == sign; i += step {
		list = append(list, l.elems[i])
	}
	return NewList(list)
}

func (l *List) Attr(name string) (Value, error) { return builtinAttr(l, name, listMethods) }
func (l *List) AttrNames() []string             { return builtinAttrNames(listMethods) }

func (l *List) Iterate() Iterator {
	if !l.frozen {
		l.itercount++
	}
	return &listIterator{l: l}
}

func (x *List) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*List)
	// It's tempting to check x == y as an optimization here,
	// but wrong because a list containing NaN is not equal to itself.
	return sliceCompare(op, x.elems, y.elems, depth)
}

func sliceCompare(op syntax.Token, x, y []Value, depth int) (bool, error) {
	// Fast path: check length.
	if len(x) != len(y) && (op == syntax.EQL || op == syntax.NEQ) {
		return op == syntax.NEQ, nil
	}

	// Find first element that is not equal in both lists.
	for i := 0; i < len(x) && i < len(y); i++ {
		if eq, err := EqualDepth(x[i], y[i], depth-1); err != nil {
			return false, err
		} else if !eq {
			switch op {
			case syntax.EQL:
				return false, nil
			case syntax.NEQ:
				return true, nil
			default:
				return CompareDepth(op, x[i], y[i], depth-1)
			}
		}
	}

	return threeway(op, len(x)-len(y)), nil
}

type listIterator struct {
	l *List
	i int
}

func (it *listIterator) Next(p *Value) bool {
	if it.i < it.l.Len() {
		*p = it.l.elems[it.i]
		it.i++
		return true
	}
	return false
}

func (it *listIterator) Done() {
	if !it.l.frozen {
		it.l.itercount--
	}
}

func (l *List) SetIndex(i int, v Value) error {
	if err := l.checkMutable("assign to element of", false); err != nil {
		return err
	}
	l.elems[i] = v
	return nil
}

func (l *List) Append(v Value) error {
	if err := l.checkMutable("append to", true); err != nil {
		return err
	}
	l.elems = append(l.elems, v)
	return nil
}

func (l *List) Clear() error {
	if err := l.checkMutable("clear", true); err != nil {
		return err
	}
	for i := range l.elems {
		l.elems[i] = nil // aid GC
	}
	l.elems = l.elems[:0]
	return nil
}

// A Tuple represents a Skylark tuple value.
type Tuple []Value

func (t Tuple) Len() int          { return len(t) }
func (t Tuple) Index(i int) Value { return t[i] }

func (t Tuple) Slice(start, end, step int) Value {
	if step == 1 {
		return t[start:end]
	}

	sign := signum(step)
	var tuple Tuple
	for i := start; signum(end-i) == sign; i += step {
		tuple = append(tuple, t[i])
	}
	return tuple
}

func (t Tuple) Iterate() Iterator { return &tupleIterator{elems: t} }
func (t Tuple) Freeze() {
	for _, elem := range t {
		elem.Freeze()
	}
}
func (t Tuple) String() string { return toString(t) }
func (t Tuple) Type() string   { return "tuple" }
func (t Tuple) Truth() Bool    { return len(t) > 0 }

func (x Tuple) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(Tuple)
	return sliceCompare(op, x, y, depth)
}

func (t Tuple) Hash() (uint32, error) {
	// Use same algorithm as Python.
	var x, mult uint32 = 0x345678, 1000003
	for _, elem := range t {
		y, err := elem.Hash()
		if err != nil {
			return 0, err
		}
		x = x ^ y*mult
		mult += 82520 + uint32(len(t)+len(t))
	}
	return x, nil
}

type tupleIterator struct{ elems Tuple }

func (it *tupleIterator) Next(p *Value) bool {
	if len(it.elems) > 0 {
		*p = it.elems[0]
		it.elems = it.elems[1:]
		return true
	}
	return false
}

func (it *tupleIterator) Done() {}

// A Set represents a Skylark set value.
type Set struct {
	ht hashtable // values are all None
}

func (s *Set) Delete(k Value) (found bool, err error) { _, found, err = s.ht.delete(k); return }
func (s *Set) Clear() error                           { return s.ht.clear() }
func (s *Set) Has(k Value) (found bool, err error)    { _, found, err = s.ht.lookup(k); return }
func (s *Set) Insert(k Value) error                   { return s.ht.insert(k, None) }
func (s *Set) Len() int                               { return int(s.ht.len) }
func (s *Set) Iterate() Iterator                      { return s.ht.iterate() }
func (s *Set) String() string                         { return toString(s) }
func (s *Set) Type() string                           { return "set" }
func (s *Set) elems() []Value                         { return s.ht.keys() }
func (s *Set) Freeze()                                { s.ht.freeze() }
func (s *Set) Hash() (uint32, error)                  { return 0, fmt.Errorf("unhashable type: set") }
func (s *Set) Truth() Bool                            { return s.Len() > 0 }

func (s *Set) Attr(name string) (Value, error) { return builtinAttr(s, name, setMethods) }
func (s *Set) AttrNames() []string             { return builtinAttrNames(setMethods) }

func (x *Set) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*Set)
	switch op {
	case syntax.EQL:
		ok, err := setsEqual(x, y, depth)
		return ok, err
	case syntax.NEQ:
		ok, err := setsEqual(x, y, depth)
		return !ok, err
	default:
		return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
	}
}

func setsEqual(x, y *Set, depth int) (bool, error) {
	if x.Len() != y.Len() {
		return false, nil
	}
	for _, elem := range x.elems() {
		if found, _ := y.Has(elem); !found {
			return false, nil
		}
	}
	return true, nil
}

func (s *Set) Union(iter Iterator) (Value, error) {
	set := new(Set)
	for _, elem := range s.elems() {
		set.Insert(elem) // can't fail
	}
	var x Value
	for iter.Next(&x) {
		if err := set.Insert(x); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// toString returns the string form of value v.
// It may be more efficient than v.String() for larger values.
func toString(v Value) string {
	var buf bytes.Buffer
	path := make([]Value, 0, 4)
	writeValue(&buf, v, path)
	return buf.String()
}

// path is the list of *List and *Dict values we're currently printing.
// (These are the only potentially cyclic structures.)
func writeValue(out *bytes.Buffer, x Value, path []Value) {
	switch x := x.(type) {
	case nil:
		out.WriteString("<nil>") // indicates a bug

	case NoneType:
		out.WriteString("None")

	case Int:
		out.WriteString(x.String())

	case Bool:
		if x {
			out.WriteString("True")
		} else {
			out.WriteString("False")
		}

	case String:
		fmt.Fprintf(out, "%q", string(x))

	case *List:
		out.WriteByte('[')
		if pathContains(path, x) {
			out.WriteString("...") // list contains itself
		} else {
			for i, elem := range x.elems {
				if i > 0 {
					out.WriteString(", ")
				}
				writeValue(out, elem, append(path, x))
			}
		}
		out.WriteByte(']')

	case Tuple:
		out.WriteByte('(')
		for i, elem := range x {
			if i > 0 {
				out.WriteString(", ")
			}
			writeValue(out, elem, path)
		}
		if len(x) == 1 {
			out.WriteByte(',')
		}
		out.WriteByte(')')

	case *Function:
		fmt.Fprintf(out, "<function %s>", x.Name())

	case *Builtin:
		if x.recv != nil {
			fmt.Fprintf(out, "<built-in method %s of %s value>", x.Name(), x.recv.Type())
		} else {
			fmt.Fprintf(out, "<built-in function %s>", x.Name())
		}

	case *Dict:
		out.WriteByte('{')
		if pathContains(path, x) {
			out.WriteString("...") // dict contains itself
		} else {
			sep := ""
			for _, item := range x.Items() {
				k, v := item[0], item[1]
				out.WriteString(sep)
				writeValue(out, k, path)
				out.WriteString(": ")
				writeValue(out, v, append(path, x)) // cycle check
				sep = ", "
			}
		}
		out.WriteByte('}')

	case *Set:
		out.WriteString("set([")
		for i, elem := range x.elems() {
			if i > 0 {
				out.WriteString(", ")
			}
			writeValue(out, elem, path)
		}
		out.WriteString("])")

	default:
		out.WriteString(x.String())
	}
}

func pathContains(path []Value, x Value) bool {
	for _, y := range path {
		if x == y {
			return true
		}
	}
	return false
}

const maxdepth = 10

// Equal reports whether two Skylark values are equal.
func Equal(x, y Value) (bool, error) {
	if x, ok := x.(String); ok {
		return x == y, nil // fast path for an important special case
	}
	return EqualDepth(x, y, maxdepth)
}

// EqualDepth reports whether two Skylark values are equal.
//
// Recursive comparisons by implementations of Value.CompareSameType
// should use EqualDepth to prevent infinite recursion.
func EqualDepth(x, y Value, depth int) (bool, error) {
	return CompareDepth(syntax.EQL, x, y, depth)
}

// Compare compares two Skylark values.
// The comparison operation must be one of EQL, NEQ, LT, LE, GT, or GE.
// Compare returns an error if an ordered comparison was
// requested for a type that does not support it.
//
// Recursive comparisons by implementations of Value.CompareSameType
// should use CompareDepth to prevent infinite recursion.
func Compare(op syntax.Token, x, y Value) (bool, error) {
	return CompareDepth(op, x, y, maxdepth)
}

// CompareDepth compares two Skylark values.
// The comparison operation must be one of EQL, NEQ, LT, LE, GT, or GE.
// CompareDepth returns an error if an ordered comparison was
// requested for a pair of values that do not support it.
//
// The depth parameter limits the maximum depth of recursion
// in cyclic data structures.
func CompareDepth(op syntax.Token, x, y Value, depth int) (bool, error) {
	if depth < 1 {
		return false, fmt.Errorf("comparison exceeded maximum recursion depth")
	}
	if sameType(x, y) {
		if xcomp, ok := x.(Comparable); ok {
			return xcomp.CompareSameType(op, y, depth)
		}

		// use identity comparison
		switch op {
		case syntax.EQL:
			return x == y, nil
		case syntax.NEQ:
			return x != y, nil
		}
		return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
	}

	// different types

	// int/float ordered comparisons
	switch x := x.(type) {
	case Int:
		if y, ok := y.(Float); ok {
			if y != y {
				return false, nil // y is NaN
			}
			var cmp int
			if !math.IsInf(float64(y), 0) {
				cmp = x.rational().Cmp(y.rational()) // y is finite
			} else if y > 0 {
				cmp = -1 // y is +Inf
			} else {
				cmp = +1 // y is -Inf
			}
			return threeway(op, cmp), nil
		}
	case Float:
		if y, ok := y.(Int); ok {
			if x != x {
				return false, nil // x is NaN
			}
			var cmp int
			if !math.IsInf(float64(x), 0) {
				cmp = x.rational().Cmp(y.rational()) // x is finite
			} else if x > 0 {
				cmp = -1 // x is +Inf
			} else {
				cmp = +1 // x is -Inf
			}
			return threeway(op, cmp), nil
		}
	}

	// All other values of different types compare unequal.
	switch op {
	case syntax.EQL:
		return false, nil
	case syntax.NEQ:
		return true, nil
	}
	return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
}

func sameType(x, y Value) bool {
	return reflect.TypeOf(x) == reflect.TypeOf(y) || x.Type() == y.Type()
}

// threeway interprets a three-way comparison value cmp (-1, 0, +1)
// as a boolean comparison (e.g. x < y).
func threeway(op syntax.Token, cmp int) bool {
	switch op {
	case syntax.EQL:
		return cmp == 0
	case syntax.NEQ:
		return cmp != 0
	case syntax.LE:
		return cmp <= 0
	case syntax.LT:
		return cmp < 0
	case syntax.GE:
		return cmp >= 0
	case syntax.GT:
		return cmp > 0
	}
	panic(op)
}

func b2i(b bool) int {
	if b {
		return 1
	} else {
		return 0
	}
}

// Len returns the length of a string or sequence value,
// and -1 for all others.
//
// Warning: Len(x) >= 0 does not imply Iterate(x) != nil.
// A string has a known length but is not directly iterable.
func Len(x Value) int {
	switch x := x.(type) {
	case String:
		return x.Len()
	case Sequence:
		return x.Len()
	}
	return -1
}

// Iterate return a new iterator for the value if iterable, nil otherwise.
// If the result is non-nil, the caller must call Done when finished with it.
//
// Warning: Iterate(x) != nil does not imply Len(x) >= 0.
// Some iterables may have unknown length.
func Iterate(x Value) Iterator {
	if x, ok := x.(Iterable); ok {
		return x.Iterate()
	}
	return nil
}
