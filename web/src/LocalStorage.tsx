import React from "react"

export type LocalStorageContext = {
  set: (key: string, value: any) => void
  get: <T extends {}>(key: string) => T | null
}

export const localStorageContext = React.createContext<LocalStorageContext>({
  set: (key: string, value: any): void => {},
  get: <T extends {}>(key: string): T | null => null,
})

export function makeKey(tiltfileKey: string, key: string): string {
  return "tilt-" + JSON.stringify({ tiltfileKey: tiltfileKey, key: key })
}

// Provides access localStorage, but namespaced by `tiltfileKey`
// Also handles serialization and typing.
export function LocalStorageContextProvider(
  props: React.PropsWithChildren<{ tiltfileKey: string }>
) {
  let tiltfileKey = props.tiltfileKey

  let set = (key: string, value: any): void => {
    localStorage.setItem(makeKey(tiltfileKey, key), JSON.stringify(value))
  }

  let get = <T extends {}>(key: string): T | null => {
    let lsk = makeKey(tiltfileKey, key)
    let json = localStorage.getItem(lsk)
    if (!json) {
      return null
    }

    try {
      return JSON.parse(json)
    } catch (e) {
      console.log(
        `error parsing local storage w/ key ${lsk}, val ${json}: ${e}`
      )
      return null
    }
  }

  return (
    <localStorageContext.Provider value={{ set: set, get: get }}>
      {props.children}
    </localStorageContext.Provider>
  )
}
