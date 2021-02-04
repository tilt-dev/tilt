import React, { useContext } from "react"
import { useStorageState } from "react-storage-hooks"

export type TiltfileKey = string
export const tiltfileKeyContext = React.createContext<TiltfileKey>("unset")

export function makeKey(tiltfileKey: string, key: string): string {
  return "tilt-" + JSON.stringify({ tiltfileKey: tiltfileKey, key: key })
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
  children: (state: S, setState: (newState: S) => void) => JSX.Element
}) {
  const [state, setState] = usePersistentState(props.name, props.defaultValue)
  return props.children(state, setState)
}
