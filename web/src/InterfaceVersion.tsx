import Cookies from "js-cookie"
import React, { PropsWithChildren, useContext, useState } from "react"
import { useHistory } from "react-router"
import { usePathBuilder } from "./PathBuilder"

export type InterfaceVersion = {
  isNewDefault(): boolean
  toggleDefault(): void
}

const interfaceVersionContext = React.createContext<InterfaceVersion>({
  isNewDefault: () => false,
  toggleDefault: () => {},
})

export function useInterfaceVersion(): InterfaceVersion {
  return useContext(interfaceVersionContext)
}

export function InterfaceVersionProvider(props: PropsWithChildren<{}>) {
  let pathBuilder = usePathBuilder()
  let history = useHistory()
  let [isNew, setNew] = useState((): boolean => {
    return Cookies.get("tilt-interface-version") !== "legacy"
  })

  let isNewDefault = () => isNew
  let toggleDefault = () => {
    let newDefault = !isNew
    setNew(newDefault)

    Cookies.set("tilt-interface-version", newDefault ? "" : "legacy")
  }

  return (
    <interfaceVersionContext.Provider value={{ isNewDefault, toggleDefault }}>
      {props.children}
    </interfaceVersionContext.Provider>
  )
}

export function FakeInterfaceVersionProvider(props: PropsWithChildren<{}>) {
  let [isNew, setNew] = useState(false)

  let isNewDefault = () => isNew
  let toggleDefault = () => {
    let newDefault = !isNew
    setNew(newDefault)
    console.log(
      newDefault
        ? 'Toggle default "old ui" -> "new ui"'
        : 'Toggle default "new ui" -> "old ui"'
    )
  }

  return (
    <interfaceVersionContext.Provider value={{ isNewDefault, toggleDefault }}>
      {props.children}
    </interfaceVersionContext.Provider>
  )
}
