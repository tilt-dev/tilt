import React, { PropsWithChildren, useContext } from "react"

// The resource whose details were open last (in this tab). The Detail view
// button reopens it, and the table marks its row, so leaving the details
// and coming back feels like closing and reopening a drawer.
const lastOpenedResourceContext = React.createContext<string>("")

export function LastOpenedResourceProvider(
  props: PropsWithChildren<{ name: string }>
) {
  return (
    <lastOpenedResourceContext.Provider value={props.name}>
      {props.children}
    </lastOpenedResourceContext.Provider>
  )
}

export function useLastOpenedResource(): string {
  return useContext(lastOpenedResourceContext)
}
