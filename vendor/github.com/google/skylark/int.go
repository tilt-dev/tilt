// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package skylark

import (
	"fmt"
	"math"
	"math/big"

	"github.com/google/skylark/syntax"
)

// Int is the type of a Skylark int.
type Int struct{ bigint *big.Int }

// MakeInt returns a Skylark int for the specified signed integer.
func MakeInt(x int) Int { return MakeInt64(int64(x)) }

// MakeInt64 returns a Skylark int for the specified int64.
func MakeInt64(x int64) Int {
	if 0 <= x && x < int64(len(smallint)) {
		if !smallintok {
			panic("MakeInt64 used before initialization")
		}
		return Int{&smallint[x]}
	}
	return Int{new(big.Int).SetInt64(x)}
}

// MakeUint returns a Skylark int for the specified unsigned integer.
func MakeUint(x uint) Int { return MakeUint64(uint64(x)) }

// MakeUint64 returns a Skylark int for the specified uint64.
func MakeUint64(x uint64) Int {
	if x < uint64(len(smallint)) {
		if !smallintok {
			panic("MakeUint64 used before initialization")
		}
		return Int{&smallint[x]}
	}
	return Int{new(big.Int).SetUint64(uint64(x))}
}

var (
	smallint   [256]big.Int
	smallintok bool
	zero, one  Int
)

func init() {
	for i := range smallint {
		smallint[i].SetInt64(int64(i))
	}
	smallintok = true

	zero = MakeInt64(0)
	one = MakeInt64(1)
}

// Int64 returns the value as an int64.
// If it is not exactly representable the result is undefined and ok is false.
func (i Int) Int64() (_ int64, ok bool) {
	x, acc := bigintToInt64(i.bigint)
	if acc != big.Exact {
		return // inexact
	}
	return x, true
}

// Uint64 returns the value as a uint64.
// If it is not exactly representable the result is undefined and ok is false.
func (i Int) Uint64() (_ uint64, ok bool) {
	x, acc := bigintToUint64(i.bigint)
	if acc != big.Exact {
		return // inexact
	}
	return x, true
}

// The math/big API should provide this function.
func bigintToInt64(i *big.Int) (int64, big.Accuracy) {
	sign := i.Sign()
	if sign > 0 {
		if i.Cmp(maxint64) > 0 {
			return math.MaxInt64, big.Below
		}
	} else if sign < 0 {
		if i.Cmp(minint64) < 0 {
			return math.MinInt64, big.Above
		}
	}
	return i.Int64(), big.Exact
}

// The math/big API should provide this function.
func bigintToUint64(i *big.Int) (uint64, big.Accuracy) {
	sign := i.Sign()
	if sign > 0 {
		if i.BitLen() > 64 {
			return math.MaxUint64, big.Below
		}
	} else if sign < 0 {
		return 0, big.Above
	}
	return i.Uint64(), big.Exact
}

var (
	minint64 = new(big.Int).SetInt64(math.MinInt64)
	maxint64 = new(big.Int).SetInt64(math.MaxInt64)
)

func (i Int) String() string { return i.bigint.String() }
func (i Int) Type() string   { return "int" }
func (i Int) Freeze()        {} // immutable
func (i Int) Truth() Bool    { return i.Sign() != 0 }
func (i Int) Hash() (uint32, error) {
	var lo big.Word
	if i.bigint.Sign() != 0 {
		lo = i.bigint.Bits()[0]
	}
	return 12582917 * uint32(lo+3), nil
}
func (x Int) CompareSameType(op syntax.Token, y Value, depth int) (bool, error) {
	return threeway(op, x.bigint.Cmp(y.(Int).bigint)), nil
}

// Float returns the float value nearest i.
func (i Int) Float() Float {
	// TODO(adonovan): opt: handle common values without allocation.
	f, _ := new(big.Float).SetInt(i.bigint).Float64()
	return Float(f)
}

func (x Int) Sign() int      { return x.bigint.Sign() }
func (x Int) Add(y Int) Int  { return Int{new(big.Int).Add(x.bigint, y.bigint)} }
func (x Int) Sub(y Int) Int  { return Int{new(big.Int).Sub(x.bigint, y.bigint)} }
func (x Int) Mul(y Int) Int  { return Int{new(big.Int).Mul(x.bigint, y.bigint)} }
func (x Int) Or(y Int) Int   { return Int{new(big.Int).Or(x.bigint, y.bigint)} }
func (x Int) And(y Int) Int  { return Int{new(big.Int).And(x.bigint, y.bigint)} }
func (x Int) Xor(y Int) Int  { return Int{new(big.Int).Xor(x.bigint, y.bigint)} }
func (x Int) Not() Int       { return Int{new(big.Int).Not(x.bigint)} }
func (x Int) Lsh(y uint) Int { return Int{new(big.Int).Lsh(x.bigint, y)} }
func (x Int) Rsh(y uint) Int { return Int{new(big.Int).Rsh(x.bigint, y)} }

// Precondition: y is nonzero.
func (x Int) Div(y Int) Int {
	// http://python-history.blogspot.com/2010/08/why-pythons-integer-division-floors.html
	var quo, rem big.Int
	quo.QuoRem(x.bigint, y.bigint, &rem)
	if (x.bigint.Sign() < 0) != (y.bigint.Sign() < 0) && rem.Sign() != 0 {
		quo.Sub(&quo, one.bigint)
	}
	return Int{&quo}
}

// Precondition: y is nonzero.
func (x Int) Mod(y Int) Int {
	var quo, rem big.Int
	quo.QuoRem(x.bigint, y.bigint, &rem)
	if (x.bigint.Sign() < 0) != (y.bigint.Sign() < 0) && rem.Sign() != 0 {
		rem.Add(&rem, y.bigint)
	}
	return Int{&rem}
}

func (i Int) rational() *big.Rat { return new(big.Rat).SetInt(i.bigint) }

// AsInt32 returns the value of x if is representable as an int32.
func AsInt32(x Value) (int, error) {
	i, ok := x.(Int)
	if !ok {
		return 0, fmt.Errorf("got %s, want int", x.Type())
	}
	if i.bigint.BitLen() <= 32 {
		v := i.bigint.Int64()
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			return int(v), nil
		}
	}
	return 0, fmt.Errorf("%s out of range", i)
}

// NumberToInt converts a number x to an integer value.
// An int is returned unchanged, a float is truncated towards zero.
// NumberToInt reports an error for all other values.
func NumberToInt(x Value) (Int, error) {
	switch x := x.(type) {
	case Int:
		return x, nil
	case Float:
		f := float64(x)
		if math.IsInf(f, 0) {
			return zero, fmt.Errorf("cannot convert float infinity to integer")
		} else if math.IsNaN(f) {
			return zero, fmt.Errorf("cannot convert float NaN to integer")
		}
		return finiteFloatToInt(x), nil

	}
	return zero, fmt.Errorf("cannot convert %s to int", x.Type())
}

// finiteFloatToInt converts f to an Int, truncating towards zero.
// f must be finite.
func finiteFloatToInt(f Float) Int {
	var i big.Int
	if math.MinInt64 <= f && f <= math.MaxInt64 {
		// small values
		i.SetInt64(int64(f))
	} else {
		rat := f.rational()
		if rat == nil {
			panic(f) // non-finite
		}
		i.Div(rat.Num(), rat.Denom())
	}
	return Int{&i}
}
