import React, { PropsWithChildren, useContext, useState } from "react"
import { AnalyticsAction, incr } from "./analytics"

export type SidebarContext = {
  isOpen: boolean
  toggleIsOpen: () => void
}

const sidebarContext = React.createContext<SidebarContext>({
  isOpen: true,
  toggleIsOpen: () => {},
})

export function useSidebarContext(): SidebarContext {
  return useContext(sidebarContext)
}

export function SidebarContextProvider(props: PropsWithChildren<{}>) {
  const [isOpen, setIsOpen] = useState<boolean>(true)

  function toggleIsOpen() {
    setIsOpen(!isOpen)
    incr("ui.web.sidebarToggle", {
      action: AnalyticsAction.SidebarToggle,
    })
  }

  return (
    <sidebarContext.Provider
      value={{
        isOpen: isOpen,
        toggleIsOpen: toggleIsOpen,
      }}
    >
      {props.children}
    </sidebarContext.Provider>
  )
}
