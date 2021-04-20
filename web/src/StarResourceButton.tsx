import React from "react"
import styled from "styled-components"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { useStarredResources } from "./StarredResourcesContext"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export const StarResourceButtonRoot = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  padding: 0;
  background-color: transparent;
  align-items: center;
`
let StarIcon = styled(StarSvg)`
  width: ${SizeUnit(1.0 / 3)};
  height: ${SizeUnit(1.0 / 3)};
`
let ActiveStarIcon = styled(StarIcon)`
  transition: transform ${AnimDuration.short} ease;
  fill: ${Color.grayLight};

  ${StarResourceButtonRoot}:hover & {
    fill: ${Color.blue};
  }
`

let InactiveStarIcon = styled(StarIcon)`
  transition: fill ${AnimDuration.default} linear,
    opacity ${AnimDuration.short} linear;
  opacity: 0;

  .u-showStarOnHover:hover &,
  ${StarResourceButtonRoot}:focus &,
  ${StarResourceButtonRoot}.u-persistShow & {
    fill: ${Color.grayLight};
    opacity: 1;
  }

  ${StarResourceButtonRoot}:hover & {
    fill: ${Color.blue};
    opacity: 1;
  }
`

type StarResourceButtonProps = {
  resourceName: string
  persistShow?: boolean
  analyticsName: string
}

export default function StarResourceButton(
  props: StarResourceButtonProps
): JSX.Element {
  let ctx = useStarredResources()
  let { resourceName, persistShow } = props
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

  function onClick(e: any) {
    e.preventDefault()
    e.stopPropagation()
    if (isStarred) {
      ctx.unstarResource(resourceName)
    } else {
      ctx.starResource(resourceName)
    }
  }

  let className = ""
  if (persistShow) {
    className = "u-persistShow"
  }
  return (
    <StarResourceButtonRoot
      title={title}
      onClick={onClick}
      className={className}
      analyticsName={props.analyticsName}
      analyticsTags={{ newStarState: (!isStarred).toString() }}
    >
      {icon}
    </StarResourceButtonRoot>
  )
}
