package fsevent

import (
	"time"

	"github.com/tilt-dev/tilt/internal/watch"
)

// BufferMinRestDuration bundles many file changes together by waiting a short amount of time after a file
// change before emitting an event to the channel.
//
// 200ms is not the result of any kind of research or experimentation
// it might end up being a significant part of deployment delay, if we get the total latency <2s
// it might also be long enough that it misses some changes if the user has some operation involving a large file
//   (e.g., a binary dependency in git), but that's hopefully less of a problem since we'd get it in the next build
const BufferMinRestDuration = 200 * time.Millisecond

// BufferMaxDuration prevents excessive delays when bundling together file changes by emitting an event to the
// channel if the threshold is reached even if new file changes are still coming in.
const BufferMaxDuration = 10 * time.Second

// Coalesce makes an attempt to read some events from `eventChan` so that multiple file changes
// that happen at the same time from the user's perspective are grouped together.
func Coalesce(timerMaker TimerMaker, eventChan <-chan watch.FileEvent) <-chan []watch.FileEvent {
	ret := make(chan []watch.FileEvent)
	go func() {
		defer close(ret)

		for {
			event, ok := <-eventChan
			if !ok {
				return
			}
			events := []watch.FileEvent{event}

			// keep grabbing changes until we've gone `BufferMinRestDuration` without seeing a change
			minRestTimer := timerMaker(BufferMinRestDuration)

			// but if we go too long before seeing a break (e.g., a process is constantly writing logs to that dir)
			// then just send what we've got
			timeout := timerMaker(BufferMaxDuration)

			done := false
			channelClosed := false
			for !done && !channelClosed {
				select {
				case event, ok := <-eventChan:
					if !ok {
						channelClosed = true
					} else {
						minRestTimer = timerMaker(BufferMinRestDuration)
						events = append(events, event)
					}
				case <-minRestTimer:
					done = true
				case <-timeout:
					done = true
				}
			}
			if len(events) > 0 {
				ret <- events
			}

			if channelClosed {
				return
			}
		}

	}()
	return ret
}
