import React from "react"
import styled from "styled-components"
import { Tags } from "./analytics"
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
`

let StyledStarSvg = styled(StarSvg)`
  width: ${SizeUnit(0.4)};
  height: ${SizeUnit(0.4)};
  padding: ${SizeUnit(0.5)};
  transition: transform ${AnimDuration.short} ease,
    opacity ${AnimDuration.short} ease;

  &:active {
    transform: scale(1.2);
  }
  &.is-starred {
    fill: ${Color.blue};
  }
  &.is-unstarred {
    opacity: 0;
    fill: ${Color.grayLight};
  }
  &.is-unstarred:hover,
  ${StyledTableStarResourceButton}:focus &.is-unstarred {
    opacity: 1;
  }
`

type StarResourceButtonProps = {
  resourceName: string
  analyticsName: string
  ctx: StarredResourcesContext
  analyticsTags: Tags
}

export default function OverviewTableStarResourceButton(
  props: StarResourceButtonProps
): JSX.Element {
  let { ctx, resourceName, analyticsTags } = props
  let isStarred =
    ctx.starredResources && ctx.starredResources.includes(resourceName)

  let icon: JSX.Element
  let classes = ""
  let title: string

  if (isStarred) {
    title = "Remove Star"
    classes = "is-starred"
  } else {
    title = "Star this Resource"
    classes = "is-unstarred"
  }

  return (
    <StyledTableStarResourceButton
      title={title}
      onClick={() => ctx.toggleStarResource(resourceName)}
      analyticsName={props.analyticsName}
      analyticsTags={{
        newStarState: (!isStarred).toString(),
        ...analyticsTags,
      }}
    >
      <StyledStarSvg className={classes} />
    </StyledTableStarResourceButton>
  )
}
