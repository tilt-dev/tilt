import React, { Dispatch, SetStateAction, useContext } from "react"
import { useStorageState } from "react-storage-hooks"

export type TiltfileKey = string
export const tiltfileKeyContext = React.createContext<TiltfileKey>("unset")

export function makeKey(tiltfileKey: string, key: string): string {
  return "tilt-" + JSON.stringify({ tiltfileKey: tiltfileKey, key: key })
}

export type accessor<S> = {
  get: () => S | null
  set: (s: S) => void
}

export function accessorsForTesting<S>(name: string) {
  const key = makeKey("test", name)
  function get(): S | null {
    const v = localStorage.getItem(key)
    if (!v) {
      return null
    }
    return JSON.parse(v) as S
  }

  function set(s: S): void {
    localStorage.setItem(key, JSON.stringify(s))
  }

  return {
    get: get,
    set: set,
  }
}

// Like `useState`, but backed by localStorage
// options:
// - maybeUpgradeSavedState: transforms any state read from storage - allows, e.g., filling in default values for
//                           fields added since the state was saved
// - keyedByTiltfile: this state is saved per tiltfile - defaults to true
export function usePersistentState<S>(
  name: string,
  defaultValue: S,
  options?: {
    maybeUpgradeSavedState?: (state: S) => S
    keyedByTiltfile?: boolean
  }
): [state: S, setState: Dispatch<SetStateAction<S>>] {
  let state: S
  let setState: Dispatch<SetStateAction<S>>
  if (options?.keyedByTiltfile !== false) {
    const tiltfileKey = useContext<TiltfileKey>(tiltfileKeyContext)

    ;[state, setState] = useStorageState<S>(
      localStorage,
      makeKey(tiltfileKey, name),
      defaultValue
    )
  } else {
    ;[state, setState] = useStorageState<S>(localStorage, name, defaultValue)
  }

  if (options?.maybeUpgradeSavedState) {
    state = options.maybeUpgradeSavedState(state)
  }
  return [state, setState]
}

export function PersistentStateProvider<S>(props: {
  name: string
  defaultValue: S
  maybeUpgradeSavedState?: (state: S) => S
  keyedByTiltfile?: boolean
  children: (state: S, setState: Dispatch<SetStateAction<S>>) => JSX.Element
}) {
  let [state, setState] = usePersistentState(props.name, props.defaultValue, {
    maybeUpgradeSavedState: props.maybeUpgradeSavedState,
    keyedByTiltfile: props.keyedByTiltfile,
  })
  return props.children(state, setState)
}
