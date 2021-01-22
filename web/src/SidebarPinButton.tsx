import React from "react"
import styled from "styled-components"
import { ReactComponent as PinResourceFilledSvg } from "./assets/svg/pin.svg"
import { useSidebarPin } from "./SidebarPin"
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

  .u-showPinOnHover:hover & {
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

export default function SidebarPinButton(props: {
  resourceName: string
}): JSX.Element {
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
