//+build linux darwin dragonfly freebsd netbsd solaris
//+build amd64 arm64,!darwin mips64x ppc64x loong64

package starlark

// This file defines an optimized Int implementation for 64-bit machines
// running POSIX. It reserves a 4GB portion of the address space using
// mmap and represents int32 values as addresses within that range. This
// disambiguates int32 values from *big.Int pointers, letting all Int
// values be represented as an unsafe.Pointer, so that Int-to-Value
// interface conversion need not allocate.

// Although iOS (arm64,darwin) claims to be a POSIX-compliant,
// it limits each process to about 700MB of virtual address space,
// which defeats the optimization.
//
// TODO(golang.org/issue/38485): darwin,arm64 may refer to macOS in the future.
// Update this when there are distinct GOOS values for macOS, iOS, and other Apple
// operating systems on arm64.
//
// This optimization is disabled on OpenBSD, because its default
// ulimit for virtual memory is a measly GB or so.

// An alternative approach to this optimization would be to embed the
// int32 values in pointers using odd values, which can be distinguished
// from (even) *big.Int pointers. However, the Go runtime does not allow
// user programs to manufacture pointers to arbitrary locations such as
// within the zero page, or non-span, non-mmap, non-stack locations,
// and it may panic if it encounters them; see Issue #382.

import (
	"log"
	"math"
	"math/big"
	"unsafe"

	"golang.org/x/sys/unix"
)

// intImpl represents a union of (int32, *big.Int) in a single pointer,
// so that Int-to-Value conversions need not allocate.
//
// The pointer is either a *big.Int, if the value is big, or a pointer into a
// reserved portion of the address space (smallints), if the value is small.
//
// See int_generic.go for the basic representation concepts.
type intImpl unsafe.Pointer

// get returns the (small, big) arms of the union.
func (i Int) get() (int64, *big.Int) {
	ptr := uintptr(i.impl)
	if ptr >= smallints && ptr < smallints+1<<32 {
		return math.MinInt32 + int64(ptr-smallints), nil
	}
	return 0, (*big.Int)(i.impl)
}

// Precondition: math.MinInt32 <= x && x <= math.MaxInt32
func makeSmallInt(x int64) Int {
	return Int{intImpl(uintptr(x-math.MinInt32) + smallints)}
}

// Precondition: x cannot be represented as int32.
func makeBigInt(x *big.Int) Int { return Int{intImpl(x)} }

// smallints is the base address of a 2^32 byte memory region.
// Pointers to addresses in this region represent int32 values.
// We assume smallints is not at the very top of the address space.
var smallints = reserveAddresses(1 << 32)

func reserveAddresses(len int) uintptr {
	b, err := unix.Mmap(-1, 0, len, unix.PROT_READ, unix.MAP_PRIVATE|unix.MAP_ANON)
	if err != nil {
		log.Fatalf("mmap: %v", err)
	}
	return uintptr(unsafe.Pointer(&b[0]))
}
