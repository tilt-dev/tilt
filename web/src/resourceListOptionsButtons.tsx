import React, { useCallback, useMemo } from "react"
import styled from "styled-components"
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

export function ExpandButton(props: { disabled: boolean }) {
  let { expandAll } = useResourceGroups()
  return (
    <ExpandButtonRoot
      title={"Expand All"}
      variant={"text"}
      onClick={expandAll}
      disabled={props.disabled}
    >
      <ExpandSvg width="16px" height="16px" />
    </ExpandButtonRoot>
  )
}

export function CollapseButton(props: { groups: string[]; disabled: boolean }) {
  let { collapseAll } = useResourceGroups()
  let { groups } = props

  let onClick = useCallback(() => {
    collapseAll(groups)
  }, [groups, collapseAll])

  return (
    <CollapseButtonRoot
      title={"Collapse All"}
      variant={"text"}
      onClick={onClick}
      disabled={props.disabled}
    >
      <CollapseSvg width="16px" height="16px" />
    </CollapseButtonRoot>
  )
}
