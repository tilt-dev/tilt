import React from "react"
import styled from "styled-components"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { useSidebarPin } from "./SidebarPin"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export const PinButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  padding: 0;
  background-color: transparent;
  align-items: center;
`
let StarIcon = styled(StarSvg)`
  width: ${SizeUnit(1.0 / 3)};
  height: ${SizeUnit(1.0 / 3)};
`
let PinnedPinIcon = styled(StarIcon)`
  transition: transform ${AnimDuration.short} ease;
  fill: ${Color.grayLight};

  ${PinButton}:hover & {
    fill: ${Color.blue};
  }
`

let UnpinnedPinIcon = styled(StarIcon)`
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
    title = "Unstar"
  } else {
    icon = <UnpinnedPinIcon />
    title = "Star"
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
    <PinButton
      title={title}
      onClick={onClick}
      className={className}
      analyticsName="ui.web.sidebarPinButton"
      analyticsTags={{ newPinState: (!isPinned).toString() }}
    >
      {icon}
    </PinButton>
  )
}
