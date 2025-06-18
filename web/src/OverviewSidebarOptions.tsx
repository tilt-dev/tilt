import { FormControlLabel } from "@material-ui/core"
import React, { useCallback, useMemo } from "react"
import styled from "styled-components"
import { Flag, useFeatures } from "./feature"
import { InstrumentedCheckbox } from "./instrumentedComponents"
import { TILTFILE_LABEL, UNLABELED_LABEL } from "./labels"
import { CollapseButton, ExpandButton } from "./resourceListOptionsButtons"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { ResourceNameFilter } from "./ResourceNameFilter"
import SidebarItem from "./SidebarItem"
import { sidebarItemIsDisabled } from "./SidebarItemView"
import {
  Color,
  Font,
  FontSize,
  mixinResetListStyle,
  SizeUnit,
} from "./style-helpers"
import { ResourceName } from "./types"
import { OverviewSidebarToggle } from "./OverviewSidebarToggle"

export const OverviewSidebarOptionsRoot = styled.div`
  display: flex;
  justify-content: space-between;
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  color: ${Color.offWhite};
  flex-direction: column;
`

const OverviewSidebarOptionsButtonRow = styled.div`
  align-items: center;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  width: 100%;
`

export const FilterOptionList = styled.ul`
  ${mixinResetListStyle};
  display: flex;
  align-items: center;
  user-select: none; /* Prevent unsightly highlighting on the label */
`

export const SidebarOptionsLabel = styled(FormControlLabel)`
  .MuiFormControlLabel-label.MuiTypography-body1 {
    line-height: 1;
  }
`

export const CheckboxToggle = styled(InstrumentedCheckbox)`
  &.MuiCheckbox-root,
  &.Mui-checked {
    color: ${Color.gray60};
  }
`

// Create a list of all the groups from the list of resources.
//
// Sadly, this logic is duplicated several times across table and sidebar,
// but there's no easy way to consolidate it right now.
function toGroups(
  items: SidebarItem[],
  hideDisabledResources: boolean
): string[] {
  let hasUnlabeled = false
  let hasTiltfile = false
  let hasLabels: { [key: string]: boolean } = {}
  items.forEach((item) => {
    const isDisabled = sidebarItemIsDisabled(item)
    if (hideDisabledResources && isDisabled) {
      return
    }

    const labels = item.labels
    const isTiltfile = item.name === ResourceName.tiltfile
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

export function OverviewSidebarOptions(props: { items?: SidebarItem[] }) {
  const features = useFeatures()
  const { options, setOptions } = useResourceListOptions()
  let toggleDisabledResources = useCallback(() => {
    setOptions({
      showDisabledResources: !options.showDisabledResources,
    })
  }, [options.showDisabledResources])

  const labelsEnabled = features.isEnabled(Flag.Labels)
  let items = props.items || []

  const hideDisabledResources = !options.showDisabledResources

  // TODO(nick): Enable/disable the expand/collapse button based
  // on whether the groups are shown and the current group state.
  let groups = useMemo(
    () => toGroups(items, hideDisabledResources),
    [items, hideDisabledResources]
  )
  const resourceFilterApplied = options.resourceNameFilter.length > 0
  const displayResourceGroups =
    labelsEnabled && groups.length && !resourceFilterApplied

  const disabledResourcesToggle = (
    <SidebarOptionsLabel
      control={
        <CheckboxToggle
          size="small"
          checked={options.showDisabledResources}
          onClick={toggleDisabledResources}
        />
      }
      label="Show disabled resources"
    />
  )

  return (
    <OverviewSidebarOptionsRoot>
      <OverviewSidebarOptionsButtonRow>
        <ResourceNameFilter />
        <OverviewSidebarToggle />
      </OverviewSidebarOptionsButtonRow>
      <OverviewSidebarOptionsButtonRow>
        <SidebarOptionsLabel
          control={
            <CheckboxToggle
              size="small"
              checked={options.alertsOnTop}
              onClick={(_e) =>
                setOptions({ alertsOnTop: !options.alertsOnTop })
              }
            />
          }
          label="Alerts on top"
        />
        <div>
          <ExpandButton disabled={!displayResourceGroups} />
          <CollapseButton groups={groups} disabled={!displayResourceGroups} />
        </div>
      </OverviewSidebarOptionsButtonRow>
      {disabledResourcesToggle}
    </OverviewSidebarOptionsRoot>
  )
}
