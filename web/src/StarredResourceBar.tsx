import React from "react"
import { useHistory } from "react-router"
import styled from "styled-components"
import { ReactComponent as DisabledSvg } from "./assets/svg/not-allowed.svg"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { useLogStore } from "./LogStore"
import { usePathBuilder } from "./PathBuilder"
import {
  ClassNameFromResourceStatus,
  disabledResourceStyleMixin,
} from "./ResourceStatus"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus, combinedStatus, runtimeStatus } from "./status"
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

const DisabledIcon = styled(DisabledSvg)`
  height: ${SizeUnit(0.5)};
  margin-right: ${SizeUnit(1 / 8)};
  width: ${SizeUnit(0.5)};
`

export const StarButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  ${StarIcon} {
    fill: ${Color.gray50};
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
  background-color: ${Color.gray30};
  padding-top: ${SizeUnit(0.125)};
  padding-bottom: ${SizeUnit(0.125)};
  position: relative; // Anchor the .isBuilding::after psuedo-element

  &:hover {
    background-color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
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
    color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
  }
  &.isPending {
    color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
    animation: ${Glow.white} 2s linear infinite;
  }
  .isSelected &.isPending {
    color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
    animation: ${Glow.dark} 2s linear infinite;
  }
  &.isNone {
    color: ${Color.gray40};
    transition: border-color ${AnimDuration.default} linear;
  }
  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray30};
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
      ${ColorRGBA(Color.gray50, ColorAlpha.translucent)},
      ${ColorRGBA(Color.gray50, ColorAlpha.translucent)} 1px,
      ${ColorRGBA(Color.black, 0)} 1px,
      ${ColorRGBA(Color.black, 0)} 6px
    );
    background-size: 200% 200%;
    animation: ${barberpole} 8s linear infinite;
  }

  &.isDisabled {
    border-color: ${ColorRGBA(Color.gray60, ColorAlpha.translucent)};

    &:not(.isSelected) {
      color: ${Color.gray60};
    }

    ${StarredResourceLabel} {
      ${disabledResourceStyleMixin}
    }
  }

  /* implement margins as padding on child buttons, to ensure the buttons consume the
     whole bounding box */
  ${StarButton} {
    margin-left: ${SizeUnit(0.25)};
    padding-right: ${SizeUnit(0.25)};
  }
  ${ResourceButton} {
    padding-left: ${SizeUnit(0.25)};
  }
`
const StarredResourceBarRoot = styled.section`
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  padding-top: ${SizeUnit(0.25)};
  padding-bottom: ${SizeUnit(0.25)};
  margin-bottom: ${SizeUnit(0.25)};
  background-color: ${Color.grayDarker};
  display: flex;

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

  const starredResourceIcon =
    props.resource.status === ResourceStatus.Disabled ? (
      <DisabledIcon role="presentation" />
    ) : null

  return (
    <TiltTooltip title={props.resource.name}>
      <StarredResourceRoot className={classes.join(" ")}>
        <ResourceButton
          onClick={() => {
            history.push(href)
          }}
          analyticsName="ui.web.starredResourceBarResource"
        >
          {starredResourceIcon}
          <StarredResourceLabel>{props.resource.name}</StarredResourceLabel>
        </ResourceButton>
        <StarButton
          onClick={onClick}
          analyticsName="ui.web.starredResourceBarUnstar"
          aria-label={`Unstar ${props.resource.name}`}
        >
          <StarIcon />
        </StarButton>
      </StarredResourceRoot>
    </TiltTooltip>
  )
}

export default function StarredResourceBar(props: StarredResourceBarProps) {
  return (
    <StarredResourceBarRoot aria-label="Starred resources">
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
  const ls = useLogStore()
  const starContext = useStarredResources()
  const namesAndStatuses = (view?.uiResources || []).flatMap((r) => {
    let name = r.metadata?.name
    if (name && starContext.starredResources.includes(name)) {
      return [
        {
          name: name,
          status: combinedStatus(buildStatus(r, ls), runtimeStatus(r, ls)),
        },
      ]
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
