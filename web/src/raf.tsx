import React, { useContext } from "react"

// requestAnimationFrame (RAF) is a general browser primitive
// for scheduling things that are only necessary for rendering.
//
// The advantage of using a RAF callback is:
// - They're paused on background tabs when the browser isn't rendering.
// - They allow you to "yield" the CPU so that if you have some long-running
//   rendering task, it doesn't make animation jittery.
//
// RafContext is a way for us to use RAF callbcks in tests.

export type RafContext = {
  requestAnimationFrame(callback: () => void): number
  cancelAnimationFrame(id: number): void
}

const rafContext = React.createContext<RafContext>({
  // By default, use the normal window schedulers.
  requestAnimationFrame: (callback: () => void) =>
    window.requestAnimationFrame(callback),
  cancelAnimationFrame: (id: number) => window.cancelAnimationFrame(id),
})

export function useRaf(): RafContext {
  return useContext(rafContext)
}

// Inject a RAF provider that runs all callbacks synchronously.
export function SyncRafProvider(props: React.PropsWithChildren<{}>) {
  let context = {
    requestAnimationFrame: (callback: () => void) => {
      callback()
      return 0
    },
    cancelAnimationFrame: () => {},
  }
  return (
    <rafContext.Provider value={context}>{props.children}</rafContext.Provider>
  )
}

export let RafProvider = rafContext.Provider

// Returns a scheduler that pauses callbacks
// until they're invoked manually by ID.
export function newFakeRaf() {
  let callbacks: any = {}
  let callbackCount = 0
  return {
    callbacks: callbacks,
    invoke: (id: number) => {
      callbacks[id].call()
      delete callbacks[id]
    },
    requestAnimationFrame: (callback: () => void) => {
      let id = ++callbackCount
      callbacks[id] = callback
      return id
    },
    cancelAnimationFrame: (id: number) => {
      delete callbacks[id]
    },
  }
}
