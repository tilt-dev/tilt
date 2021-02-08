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

// Like `useState`, but backed by localStorage and namespaced by the tiltfileKey
export function usePersistentState<S>(name: string, defaultValue: S) {
  const tiltfileKey = useContext(tiltfileKeyContext)
  return useStorageState<S>(
    localStorage,
    makeKey(tiltfileKey, name),
    defaultValue
  )
}

export function PersistentStateProvider<S>(props: {
  name: string
  defaultValue: S
  children: (state: S, setState: Dispatch<SetStateAction<S>>) => JSX.Element
}) {
  const [state, setState] = usePersistentState(props.name, props.defaultValue)
  return props.children(state, setState)
}
