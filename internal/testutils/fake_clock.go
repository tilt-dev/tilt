package testutils

import (
	"math/rand"
	"time"
)

type Clock func() time.Time

type FakeClock struct {
	Times    []time.Time
	nextTime Clock
}

func (fc *FakeClock) next() time.Time {
	ret := fc.nextTime()
	fc.Times = append(fc.Times, ret)
	return ret
}

func (fc *FakeClock) Clock() Clock {
	return fc.next
}

func NewRandomFakeClock() *FakeClock {
	return &FakeClock{
		nextTime: func() time.Time {
			d := rand.Int()
			h := rand.Int()
			return time.Date(2019, 1, d, h, 0, 0, 0, time.UTC)
		},
	}
}
