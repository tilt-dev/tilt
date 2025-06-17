import React, { PropsWithChildren, useContext, useState } from "react"

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

export function SidebarContextProvider(
  props: PropsWithChildren<{ sidebarClosedForTesting?: boolean }>
) {
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(
    !props.sidebarClosedForTesting
  )

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
