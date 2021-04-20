import React from "react"
import { useHistory } from "react-router"
import styled from "styled-components"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { usePathBuilder } from "./PathBuilder"
import { ClassNameFromResourceStatus } from "./ResourceStatus"
import { useStarredResources } from "./StarredResourcesContext"
import { combinedStatus } from "./status"
import {
  AnimDuration,
  barberpole,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  Glow,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import TiltTooltip from "./Tooltip"
import { ResourceStatus } from "./types"

export const StarredResourceLabel = styled.div`
  max-width: ${SizeUnit(4.5)};
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;

  font-size: ${FontSize.small};
  font-family: ${Font.monospace};

  user-select: none;
`
const ResourceButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  color: inherit;
  display: flex;
`
const StarIcon = styled(StarSvg)`
  height: ${SizeUnit(0.5)};
  width: ${SizeUnit(0.5)};
`
export const StarButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  ${StarIcon} {
    fill: ${Color.grayLight};
  }
  &:hover {
    ${StarIcon} {
      fill: ${Color.grayLightest};
    }
  }
`
const StarredResourceRoot = styled.div`
  border-width: 1px;
  border-style: solid;
  border-radius: ${SizeUnit(0.125)};
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  background-color: ${Color.gray};
  padding-top: ${SizeUnit(0.125)};
  padding-bottom: ${SizeUnit(0.125)};
  position: relative; // Anchor the .isBuilding::after psuedo-element

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }

  &.isWarning {
    color: ${Color.yellow};
    border-color: ${ColorRGBA(Color.yellow, ColorAlpha.translucent)};
  }
  &.isHealthy {
    color: ${Color.green};
    border-color: ${ColorRGBA(Color.green, ColorAlpha.translucent)};
  }
  &.isUnhealthy {
    color: ${Color.red};
    border-color: ${ColorRGBA(Color.red, ColorAlpha.translucent)};
  }
  &.isBuilding {
    color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
  }
  .isSelected &.isBuilding {
    color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }
  &.isPending {
    color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
    animation: ${Glow.white} 2s linear infinite;
  }
  .isSelected &.isPending {
    color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
    animation: ${Glow.dark} 2s linear infinite;
  }
  &.isNone {
    color: ${Color.grayLighter};
    transition: border-color ${AnimDuration.default} linear;
  }
  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray};
  }

  &.isBuilding::after {
    content: "";
    position: absolute;
    pointer-events: none;
    width: 100%;
    top: 0;
    bottom: 0;
    background: repeating-linear-gradient(
      225deg,
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)},
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)} 1px,
      ${ColorRGBA(Color.black, 0)} 1px,
      ${ColorRGBA(Color.black, 0)} 6px
    );
    background-size: 200% 200%;
    animation: ${barberpole} 8s linear infinite;
  }

  // implement margins as padding on child buttons, to ensure the buttons consume the
  // whole bounding box
  ${StarButton} {
    margin-left: ${SizeUnit(0.25)};
    padding-right: ${SizeUnit(0.25)};
  }
  ${ResourceButton} {
    padding-left: ${SizeUnit(0.25)};
  }
`
const StarredResourceBarRoot = styled.div`
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  padding-top: ${SizeUnit(0.25)};
  padding-bottom: ${SizeUnit(0.25)};
  margin-bottom: ${SizeUnit(0.25)};
  background-color: ${Color.grayDarker};

  ${StarredResourceRoot} {
    margin-right: ${SizeUnit(0.25)};
  }
`

export type ResourceNameAndStatus = {
  name: string
  status: ResourceStatus
}
export type StarredResourceBarProps = {
  selectedResource?: string
  resources: ResourceNameAndStatus[]
  unstar: (name: string) => void
}

export function StarredResource(props: {
  resource: ResourceNameAndStatus
  unstar: (name: string) => void
  isSelected: boolean
}) {
  const pb = usePathBuilder()
  const href = pb.encpath`/r/${props.resource.name}/overview`
  const history = useHistory()
  const onClick = (e: any) => {
    props.unstar(props.resource.name)
    e.preventDefault()
    e.stopPropagation()
  }

  let classes = [ClassNameFromResourceStatus(props.resource.status)]
  if (props.isSelected) {
    classes.push("isSelected")
  }

  return (
    <TiltTooltip title={props.resource.name}>
      <StarredResourceRoot className={classes.join(" ")}>
        <ResourceButton
          onClick={() => {
            history.push(href)
          }}
          analyticsName="ui.web.starredResourceBarResource"
        >
          <StarredResourceLabel>{props.resource.name}</StarredResourceLabel>
        </ResourceButton>
        <StarButton
          onClick={onClick}
          analyticsName="ui.web.starredResourceBarUnstar"
        >
          <StarIcon />
        </StarButton>
      </StarredResourceRoot>
    </TiltTooltip>
  )
}

export default function StarredResourceBar(props: StarredResourceBarProps) {
  return (
    <StarredResourceBarRoot>
      {props.resources.map((r) => (
        <StarredResource
          resource={r}
          key={r.name}
          unstar={props.unstar}
          isSelected={r.name === props.selectedResource}
        />
      ))}
    </StarredResourceBarRoot>
  )
}

// translates the view to a pared-down model so that `StarredResourceBar` can have a simple API for testing.
export function starredResourcePropsFromView(
  view: Proto.webviewView,
  selectedResource: string
): StarredResourceBarProps {
  const starContext = useStarredResources()
  const namesAndStatuses = (view?.resources || []).flatMap((r) => {
    if (r.name && starContext.starredResources.includes(r.name)) {
      return [{ name: r.name, status: combinedStatus(r) }]
    } else {
      return []
    }
  })
  return {
    resources: namesAndStatuses,
    unstar: starContext.unstarResource,
    selectedResource: selectedResource,
  }
}
