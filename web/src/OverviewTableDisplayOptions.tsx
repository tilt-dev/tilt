import { FormControlLabel } from "@material-ui/core"
import React, { useCallback, useMemo } from "react"
import styled from "styled-components"
import { Flag, useFeatures } from "./feature"
import { InstrumentedCheckbox } from "./instrumentedComponents"
import { getResourceLabels, TILTFILE_LABEL, UNLABELED_LABEL } from "./labels"
import { CollapseButton, ExpandButton } from "./resourceListOptionsButtons"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { resourceIsDisabled } from "./ResourceStatus"
import { Color, Font, FontSize } from "./style-helpers"
import { ResourceName, UIResource } from "./types"

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

  const hideDisabledResources = !options.showDisabledResources

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
      <FormControlLabel
        control={
          <DisplayOptionCheckbox
            size="small"
            checked={options.showDisabledResources}
            onClick={toggleDisabledResources}
          />
        }
        label="Show disabled resources"
      />
      <ExpandButton disabled={!displayResourceGroups} />
      <CollapseButton groups={groups} disabled={!displayResourceGroups} />
    </DisplayOptions>
  )
}
