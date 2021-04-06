import { Tooltip } from "@material-ui/core"
import React from "react"
import { useHistory } from "react-router"
import styled from "styled-components"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { usePathBuilder } from "./PathBuilder"
import { ClassNameFromResourceStatus } from "./ResourceStatus"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  Glow,
  SizeUnit,
} from "./style-helpers"
import { ResourceStatus } from "./types"

const StarredResourceBarRoot = styled.div`
  margin-left: ${SizeUnit(0.5)};
  margin-right: ${SizeUnit(0.5)};
`
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
export const StarButton = styled(StarSvg)`
  height: ${SizeUnit(0.5)};
  width: ${SizeUnit(0.5)};
  fill: ${Color.grayLight};
  &:hover {
    fill: ${Color.grayLightest};
  }
`
const StarredResourceRoot = styled.div`
  border-width: 1px;
  border-style: solid;
  border-radius: ${SizeUnit(0.125)};
  padding-left: ${SizeUnit(0.25)};
  padding-right: ${SizeUnit(0.25)};
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  background-color: ${Color.gray};
  position: relative;

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }

  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray};
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

  ${StarButton} {
    margin-left: ${SizeUnit(0.25)};
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
    <Tooltip title={props.resource.name}>
      <StarredResourceRoot
        className={classes.join(" ")}
        onClick={() => {
          history.push(href)
        }}
      >
        <StarredResourceLabel>{props.resource.name}</StarredResourceLabel>
        <StarButton onClick={onClick} />
      </StarredResourceRoot>
    </Tooltip>
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
