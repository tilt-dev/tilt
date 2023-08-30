import React, { useCallback, useMemo } from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { ReactComponent as CollapseSvg } from "./assets/svg/collapse.svg"
import { ReactComponent as ExpandSvg } from "./assets/svg/expand.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { useResourceGroups } from "./ResourceGroupsContext"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

const buttonStyle = `
  ${mixinResetButtonStyle};
  padding: 0 ${SizeUnit(0.25)};
  border-radius: 0;

  &:last-child {
    padding-right: ${SizeUnit(0.5)};
  }

  .fill-std {
    fill: ${Color.gray70};
    transition: fill ${AnimDuration.default} ease
  }

  &:hover .fill-std {
    fill: ${Color.blue};
  }

  &.Mui-disabled .fill-std {
    fill: ${Color.gray50};
  }
`

const ExpandButtonRoot = styled(InstrumentedButton)`
  ${buttonStyle}
`
const CollapseButtonRoot = styled(InstrumentedButton)`
  ${buttonStyle}
  border-left: 1px solid ${Color.gray50};
`

const analyticsTags = { type: AnalyticsType.Detail }

export function ExpandButton(props: {
  groups: string[]
  disabled: boolean
  analyticsType: AnalyticsType
}) {
  let { expandAll } = useResourceGroups()
  let { analyticsType, groups } = props
  let analyticsTags = useMemo(() => {
    return { type: analyticsType }
  }, [analyticsType])
  let onClick = useCallback(() => {
    expandAll(groups)
  }, [groups, expandAll])

  return (
    <ExpandButtonRoot
      title={"Expand All"}
      variant={"text"}
      onClick={onClick}
      analyticsName={"ui.web.expandAllGroups"}
      analyticsTags={analyticsTags}
      disabled={props.disabled}
    >
      <ExpandSvg width="16px" height="16px" />
    </ExpandButtonRoot>
  )
}

export function CollapseButton(props: {
  groups: string[]
  disabled: boolean
  analyticsType: AnalyticsType
}) {
  let { collapseAll } = useResourceGroups()
  let { groups, analyticsType } = props

  let onClick = useCallback(() => {
    collapseAll(groups)
  }, [groups, collapseAll])

  let analyticsTags = useMemo(() => {
    return { type: analyticsType }
  }, [analyticsType])

  return (
    <CollapseButtonRoot
      title={"Collapse All"}
      variant={"text"}
      onClick={onClick}
      analyticsName={"ui.web.collapseAllGroups"}
      analyticsTags={analyticsTags}
      disabled={props.disabled}
    >
      <CollapseSvg width="16px" height="16px" />
    </CollapseButtonRoot>
  )
}
