import React, { PropsWithChildren, useContext, useState } from "react"
import { AnalyticsAction, incr } from "./analytics"

export type SidebarContext = {
  isSidebarOpen: boolean
  setSidebarOpen: () => void
  setSidebarClosed: () => void
}

const sidebarContext = React.createContext<SidebarContext>({
  isSidebarOpen: true,
  setSidebarOpen: () => {},
  setSidebarClosed: () => {},
})

export function useSidebarContext(): SidebarContext {
  return useContext(sidebarContext)
}

export function SidebarMemoryProvider(
  props: PropsWithChildren<{ sidebarClosedForTesting?: boolean }>
) {
  const initVal = props.sidebarClosedForTesting ? false : true
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(initVal)
  function setSidebarOpen() {
    setIsSidebarOpen(true)
  }

  function setSidebarClosed() {
    setIsSidebarOpen(false)
  }

  return (
    <sidebarContext.Provider
      value={{
        isSidebarOpen: isSidebarOpen,
        setSidebarOpen: setSidebarOpen,
        setSidebarClosed: setSidebarClosed,
      }}
    >
      {props.children}
    </sidebarContext.Provider>
  )
}

export function SidebarContextProvider(props: PropsWithChildren<{}>) {
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(true)

  function setSidebarOpen() {
    setIsSidebarOpen(true)
    incr("ui.web.sidebarToggle", {
      action: AnalyticsAction.SidebarToggle,
    })
  }

  function setSidebarClosed() {
    setIsSidebarOpen(false)
    incr("ui.web.sidebarToggle", {
      action: AnalyticsAction.SidebarToggle,
    })
  }

  return (
    <sidebarContext.Provider
      value={{
        isSidebarOpen: isSidebarOpen,
        setSidebarOpen: setSidebarOpen,
        setSidebarClosed: setSidebarClosed,
      }}
    >
      {props.children}
    </sidebarContext.Provider>
  )
}
