import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as PinResourceFilledSvg } from "./assets/svg/pin-resource-filled.svg"
import { localStorageContext } from "./LocalStorage"
import { SidebarItemRoot } from "./SidebarItem"
import { Color, Width } from "./style-helpers"

let PinnedPinIcon = styled(PinResourceFilledSvg)`
  fill: ${Color.yellowLight};
`
let UnpinnedPinIcon = styled(PinResourceFilledSvg)`
  transition: fill 300ms ease;
  fill: ${Color.grayLight};
  display: none;
  ${SidebarItemRoot}:hover & {
    display: flex;
  }
  &:hover {
    fill: ${Color.yellowLight};
  }
`
let PinButton = styled.button`
  display: flex;
  cursor: pointer;
  padding: 0;
  background-position: center center;
  background-repeat: no-repeat;
  background-color: transparent;
  border: 0 none;
  width: ${Width.sidebarPinButton}px;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
`

type SidebarPinContext = {
  pinnedResources: string[]
  pinResource: (name: string) => void
  unpinResource: (name: string) => void
}

export const sidebarPinContext = React.createContext<SidebarPinContext>({
  pinnedResources: [],
  pinResource: (s) => {},
  unpinResource: (s) => {},
})

export function SidebarPinContextProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  let lsc = useContext(localStorageContext)

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
  let ctx = useContext(sidebarPinContext)
  let isPinned =
    ctx.pinnedResources && ctx.pinnedResources.includes(props.resourceName)

  let icon: JSX.Element
  let onClick: (resourceName: string) => void
  let title: string

  if (isPinned) {
    icon = <PinnedPinIcon />
    onClick = ctx.unpinResource
    title = "unpin"
  } else {
    icon = <UnpinnedPinIcon />
    onClick = ctx.pinResource
    title = "pin"
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
