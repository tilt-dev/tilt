import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as PinResourceFilledSvg } from "./assets/svg/pin.svg"
import { useLocalStorageContext } from "./LocalStorage"
import { SidebarItemRoot } from "./SidebarItem"
import { AnimDuration, Color, Width } from "./style-helpers"

let PinButton = styled.button`
  display: flex;
  cursor: pointer;
  padding: 0;
  background-color: transparent;
  border: 0 none;
  width: ${Width.sidebarPinButton}px;
  align-items: center;
  justify-content: center;
`

let PinnedPinIcon = styled(PinResourceFilledSvg)`
  transition: transform ${AnimDuration.short} ease;
  fill: ${Color.blue};

  ${PinButton}:active & {
    fill: ${Color.blueDark};
    transform: scale(1.2);
  }
`

let UnpinnedPinIcon = styled(PinResourceFilledSvg)`
  transition: fill ${AnimDuration.default} linear,
    transform ${AnimDuration.short} ease, opacity ${AnimDuration.short} linear;
  opacity: 0;

  ${SidebarItemRoot}:hover & {
    fill: ${Color.grayLight};
    opacity: 1;
  }

  ${PinButton}:hover & {
    fill: ${Color.blueDark};
    opacity: 1;
  }

  ${PinButton}:active & {
    fill: ${Color.blue};
    transform: scale(1.2);
    opacity: 1;
  }
`

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
  let lsc = useLocalStorageContext()

  const [pinnedResources, setPinnedResources] = useState<Array<string>>(
    () =>
      props.initialValueForTesting ??
      lsc.get<Array<string>>("pinned-resources") ??
      []
  )

  useEffect(() => {
    incr("ui.web.pin", {
      pinCount: pinnedResources.length.toString(),
      action: "load",
    })
    // empty deps because we only want to report the loaded pin count once per app load
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    lsc.set("pinned-resources", pinnedResources)
  }, [pinnedResources, lsc])

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

export function SidebarPinButton(props: { resourceName: string }): JSX.Element {
  let ctx = useSidebarPin()
  let isPinned =
    ctx.pinnedResources && ctx.pinnedResources.includes(props.resourceName)

  let icon: JSX.Element
  let onClick: (resourceName: string) => void
  let title: string

  if (isPinned) {
    icon = <PinnedPinIcon />
    onClick = ctx.unpinResource
    title = "Remove Pin"
  } else {
    icon = <UnpinnedPinIcon />
    onClick = ctx.pinResource
    title = "Pin to Top"
  }

  return (
    <PinButton title={title} onClick={() => onClick(props.resourceName)}>
      {icon}
    </PinButton>
  )
}

export const SidebarPinButtonSpacer = styled.div`
  width: ${Width.sidebarPinButton}px;
`
