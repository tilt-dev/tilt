import React from "react"
import styled from "styled-components"
import { ReactComponent as PinResourceFilledSvg } from "./assets/svg/pin.svg"
import { useSidebarPin } from "./SidebarPin"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"

let PinButton = styled.button`
  ${mixinResetButtonStyle}
  display: flex;
  padding: 0;
  background-color: transparent;
  align-items: center;
  justify-content: flex-start;
  margin-right: 5px;
`

let PinnedPinIcon = styled(PinResourceFilledSvg)`
  transition: transform ${AnimDuration.short} ease;
  fill: ${Color.grayLight};

  ${PinButton}:hover & {
    fill: ${Color.blue};
  }
`

let UnpinnedPinIcon = styled(PinResourceFilledSvg)`
  transition: fill ${AnimDuration.default} linear,
    opacity ${AnimDuration.short} linear;
  opacity: 0;

  .u-showPinOnHover:hover &,
  ${PinButton}:focus &,
  ${PinButton}.u-persistShow & {
    fill: ${Color.grayLight};
    opacity: 1;
  }

  ${PinButton}:hover & {
    fill: ${Color.blue};
    opacity: 1;
  }
`

type SidebarPinButtonProps = {
  resourceName: string
  persistShow?: boolean
}

export default function SidebarPinButton(
  props: SidebarPinButtonProps
): JSX.Element {
  let ctx = useSidebarPin()
  let { resourceName, persistShow } = props
  let isPinned =
    ctx.pinnedResources && ctx.pinnedResources.includes(resourceName)

  let icon: JSX.Element
  let title: string

  if (isPinned) {
    icon = <PinnedPinIcon />
    title = "Remove Pin"
  } else {
    icon = <UnpinnedPinIcon />
    title = "Pin to Top"
  }

  function onClick(e: any) {
    e.preventDefault()
    e.stopPropagation()
    if (isPinned) {
      ctx.unpinResource(resourceName)
    } else {
      ctx.pinResource(resourceName)
    }
  }

  let className = ""
  if (persistShow) {
    className = "u-persistShow"
  }
  return (
    <PinButton title={title} onClick={onClick} className={className}>
      {icon}
    </PinButton>
  )
}
