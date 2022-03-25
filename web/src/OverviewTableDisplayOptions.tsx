import { FormControlLabel } from "@material-ui/core"
import React, { useCallback, useMemo } from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { ReactComponent as CollapseSvg } from "./assets/svg/collapse.svg"
import { ReactComponent as ExpandSvg } from "./assets/svg/expand.svg"
import { Flag, useFeatures } from "./feature"
import {
  InstrumentedButton,
  InstrumentedCheckbox,
} from "./instrumentedComponents"
import { getResourceLabels, TILTFILE_LABEL, UNLABELED_LABEL } from "./labels"
import { useResourceGroups } from "./ResourceGroupsContext"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { resourceIsDisabled } from "./ResourceStatus"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { ResourceName, UIResource } from "./types"

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

const analyticsTags = { type: AnalyticsType.Grid }

function ExpandButton(props: { disabled: boolean }) {
  let { expandAll } = useResourceGroups()
  return (
    <ExpandButtonRoot
      title={"Expand All"}
      variant={"text"}
      onClick={expandAll}
      analyticsName={"ui.web.expandAllGroups"}
      analyticsTags={analyticsTags}
      disabled={props.disabled}
    >
      <ExpandSvg width="16px" height="16px" />
    </ExpandButtonRoot>
  )
}

function CollapseButton(props: { groups: string[]; disabled: boolean }) {
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
      analyticsName={"ui.web.collapseAllGroups"}
      analyticsTags={analyticsTags}
      disabled={props.disabled}
    >
      <CollapseSvg width="16px" height="16px" />
    </CollapseButtonRoot>
  )
}

const DisplayOptions = styled.div`
  margin-left: auto;
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
`

const DisplayOptionCheckbox = styled(InstrumentedCheckbox)`
  &.MuiCheckbox-root,
  &.Mui-checked {
    color: ${Color.gray60};
  }
`

// Create a list of all the groups from the list of resources.
//
// Sadly, this logic is duplicated across table and sidebar,
// but there's no easy way to consolidate it right now.
function toGroups(
  resources: UIResource[],
  hideDisabledResources: boolean
): string[] {
  let hasUnlabeled = false
  let hasTiltfile = false
  let hasLabels: { [key: string]: boolean } = {}
  resources.forEach((r) => {
    const resourceDisabled = resourceIsDisabled(r)
    if (hideDisabledResources && resourceDisabled) {
      return
    }

    const labels = getResourceLabels(r)
    const isTiltfile = r.metadata?.name === ResourceName.tiltfile
    if (labels.length) {
      labels.forEach((label) => {
        hasLabels[label] = true
      })
    } else if (isTiltfile) {
      hasTiltfile = true
    } else {
      hasUnlabeled = true
    }
  })

  let groups = Object.keys(hasLabels)
  if (groups.length) {
    if (hasTiltfile) {
      groups.push(TILTFILE_LABEL)
    }
    if (hasUnlabeled) {
      groups.push(UNLABELED_LABEL)
    }
  }
  return groups
}

export function OverviewTableDisplayOptions(props: {
  resources?: UIResource[]
}) {
  const features = useFeatures()
  const { options, setOptions } = useResourceListOptions()
  let toggleDisabledResources = useCallback(() => {
    setOptions({
      showDisabledResources: !options.showDisabledResources,
    })
  }, [options.showDisabledResources])

  const labelsEnabled = features.isEnabled(Flag.Labels)
  let resources = props.resources || []

  const hideDisabledResources =
    !features.isEnabled(Flag.DisableResources) || !options.showDisabledResources

  // TODO(nick): Enable/disable the expand/collapse button based
  // on whether the groups are shown and the current group state.
  let groups = useMemo(
    () => toGroups(resources, hideDisabledResources),
    [resources, hideDisabledResources]
  )
  const resourceFilterApplied = options.resourceNameFilter.length > 0
  const displayResourceGroups =
    labelsEnabled && groups.length && !resourceFilterApplied

  return (
    <DisplayOptions>
      {features.isEnabled(Flag.DisableResources) ? (
        <FormControlLabel
          control={
            <DisplayOptionCheckbox
              analyticsName="ui.web.disabledResourcesToggle"
              analyticsTags={analyticsTags}
              size="small"
              checked={options.showDisabledResources}
              onClick={toggleDisabledResources}
            />
          }
          label="Show disabled resources"
        />
      ) : null}
      <ExpandButton disabled={!displayResourceGroups} />
      <CollapseButton groups={groups} disabled={!displayResourceGroups} />
    </DisplayOptions>
  )
}
