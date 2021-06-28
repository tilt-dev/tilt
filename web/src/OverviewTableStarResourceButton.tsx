import React from "react"
import styled from "styled-components"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { StarredResourcesContext } from "./StarredResourcesContext"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export const StyledTableStarResourceButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
`
let StarIcon = styled(StarSvg)`
  width: ${SizeUnit(1 / 2.5)};
  height: ${SizeUnit(1 / 2.5)};
`
let ActiveStarIcon = styled(StarIcon)`
  transition: transform ${AnimDuration.short} ease;
  fill: ${Color.grayLight};

  ${StyledTableStarResourceButton}:hover & {
    fill: ${Color.blue};
  }
`

let InactiveStarIcon = styled(StarIcon)`
  transition: fill ${AnimDuration.default} linear,
    opacity ${AnimDuration.short} linear;
  opacity: 0;

  .u-showStarOnHover:hover &,
  ${StyledTableStarResourceButton}:focus & {
    fill: ${Color.grayLight};
    opacity: 1;
  }

  ${StyledTableStarResourceButton}:hover & {
    fill: ${Color.blue};
    opacity: 1;
  }
`

type StarResourceButtonProps = {
  resourceName: string
  analyticsName: string
  ctx: StarredResourcesContext
}

export default function OverviewTableStarResourceButton(
  props: StarResourceButtonProps
): JSX.Element {
  let { ctx, resourceName } = props
  let isStarred =
    ctx.starredResources && ctx.starredResources.includes(resourceName)

  let icon: JSX.Element
  let title: string

  if (isStarred) {
    icon = <ActiveStarIcon />
    title = "Unstar"
  } else {
    icon = <InactiveStarIcon />
    title = "Star"
  }

  return (
    <StyledTableStarResourceButton
      title={title}
      onClick={() => ctx.toggleStarResource(resourceName)}
      analyticsName={props.analyticsName}
      analyticsTags={{ newStarState: (!isStarred).toString() }}
    >
      {icon}
    </StyledTableStarResourceButton>
  )
}
