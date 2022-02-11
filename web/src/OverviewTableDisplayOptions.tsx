import { FormControlLabel } from "@material-ui/core"
import React from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { Flag, useFeatures } from "./feature"
import { InstrumentedCheckbox } from "./instrumentedComponents"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { Color, Font, FontSize } from "./style-helpers"

const DisplayOptions = styled.div`
  margin-left: auto;
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
`

const DisplayOptionCheckbox = styled(InstrumentedCheckbox)`
  &.MuiCheckbox-root,
  &.Mui-checked {
    color: ${Color.gray6};
  }
`

export function OverviewTableDisplayOptions() {
  const features = useFeatures()
  const { options, setOptions } = useResourceListOptions()

  // Since the only option here is related to the Disable Resources feature,
  // don't render if the feature isn't enabled
  if (!features.isEnabled(Flag.DisableResources)) {
    return null
  }

  return (
    <DisplayOptions>
      <FormControlLabel
        control={
          <DisplayOptionCheckbox
            analyticsName="ui.web.disabledResourcesToggle"
            analyticsTags={{ type: AnalyticsType.Grid }}
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
    </DisplayOptions>
  )
}
