import { FormControlLabel } from "@material-ui/core"
import React from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { Flag, useFeatures } from "./feature"
import { InstrumentedCheckbox } from "./instrumentedComponents"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { ResourceNameFilter } from "./ResourceNameFilter"
import {
  Color,
  Font,
  FontSize,
  mixinResetListStyle,
  SizeUnit,
} from "./style-helpers"

const OverviewSidebarOptionsRoot = styled.div`
  display: flex;
  justify-content: space-between;
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  color: ${Color.offWhite};
  flex-direction: column;
`

const OverviewSidebarOptionsButtonsRoot = styled.div`
  align-items: center;
  display: flex;
  flex-direction: row;
  justify-content: flex-start;
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

export function OverviewSidebarOptions() {
  const { options, setOptions } = useResourceListOptions()
  const features = useFeatures()

  const disableResourcesEnabled = features.isEnabled(Flag.DisableResources)
  const disabledResourcesToggle = disableResourcesEnabled ? (
    <SidebarOptionsLabel
      control={
        <CheckboxToggle
          analyticsName="ui.web.disabledResourcesToggle"
          analyticsTags={{ type: AnalyticsType.Detail }}
          size="small"
          checked={options.showDisabledResources}
          onClick={(_e) =>
            setOptions({
              showDisabledResources: !options.showDisabledResources,
            })
          }
        />
      }
      label="Show disabled resources"
    />
  ) : null

  return (
    <OverviewSidebarOptionsRoot>
      <OverviewSidebarOptionsButtonsRoot>
        {disabledResourcesToggle}
        <SidebarOptionsLabel
          control={
            <CheckboxToggle
              analyticsName="ui.web.alertsOnTopToggle"
              size="small"
              checked={options.alertsOnTop}
              onClick={(_e) =>
                setOptions({ alertsOnTop: !options.alertsOnTop })
              }
            />
          }
          label="Alerts on top"
        />
      </OverviewSidebarOptionsButtonsRoot>
      <ResourceNameFilter />
    </OverviewSidebarOptionsRoot>
  )
}
