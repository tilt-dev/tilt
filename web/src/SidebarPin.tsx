import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from "react"
import { incr } from "./analytics"
import { usePersistentState } from "./LocalStorage"

type SidebarPinContext = {
  pinnedResources: string[]
  pinResource: (name: string) => void
  unpinResource: (name: string) => void
}

const sidebarPinContext = React.createContext<SidebarPinContext>({
  pinnedResources: [],
  pinResource: (s) => {},
  unpinResource: (s) => {},
})

export function useSidebarPin(): SidebarPinContext {
  return useContext(sidebarPinContext)
}

export function SidebarPinMemoryProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  const [pinnedResources, setPinnedResources] = useState<Array<string>>(
    props.initialValueForTesting || []
  )

  function pinResource(name: string) {
    setPinnedResources((prevState) => {
      return prevState.includes(name) ? prevState : [...prevState, name]
    })
  }

  function unpinResource(name: string) {
    setPinnedResources((prevState) => {
      return prevState.filter((s) => s !== name)
    })
  }

  return (
    <sidebarPinContext.Provider
      value={{ pinnedResources, pinResource, unpinResource }}
    >
      {props.children}
    </sidebarPinContext.Provider>
  )
}

export function SidebarPinContextProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  let [pinnedResources, setPinnedResources] = usePersistentState<string[]>(
    "pinned-resources",
    props.initialValueForTesting ?? []
  )

  useEffect(() => {
    incr("ui.web.pin", {
      pinCount: pinnedResources.length.toString(),
      action: "load",
    })
    // empty deps because we only want to report the loaded pin count once per app load
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function pinResource(name: string) {
    setPinnedResources((prevState) => {
      const ret = prevState.includes(name) ? prevState : [...prevState, name]
      incr("ui.web.pin", {
        pinCount: ret.length.toString(),
        action: "pin",
      })
      return ret
    })
  }

  function unpinResource(name: string) {
    setPinnedResources((prevState) => {
      const ret = prevState.filter((n) => n !== name)
      incr("ui.web.pin", {
        pinCount: ret.length.toString(),
        action: "unpin",
      })
      return ret
    })
  }

  return (
    <sidebarPinContext.Provider
      value={{ pinnedResources, pinResource, unpinResource }}
    >
      {props.children}
    </sidebarPinContext.Provider>
  )
}
