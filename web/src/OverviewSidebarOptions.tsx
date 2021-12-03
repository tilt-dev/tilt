import React from "react"
import styled from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import { ResourceNameFilter } from "./ResourceNameFilter"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  mixinResetListStyle,
  SizeUnit,
} from "./style-helpers"

const OverviewSidebarOptionsRoot = styled.div`
  display: flex;
  justify-content: space-between;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallester};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  color: ${Color.offWhite};
  flex-direction: column;
`

const OverviewSidebarOptionsButtonsRoot = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  justify-content: flex-end;
`

export const FilterOptionList = styled.ul`
  ${mixinResetListStyle};
  display: flex;
  align-items: center;
  user-select: none; // Prevent unsightly highlighting on the label
`

const toggleBorderRadius = "3px"

export const AlertsOnTopToggle = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  color: ${Color.grayLightest};
  background-color: ${Color.gray};
  padding: ${SizeUnit(0.125)} ${SizeUnit(0.25)};
  border-radius: ${toggleBorderRadius};
  font-size: 12px;

  &.is-enabled {
    color: ${Color.grayDarkest};
    background-color: ${Color.offWhite};
  }
`

export function OverviewSidebarOptions() {
  const { options, setOptions } = useResourceListOptions()

  function setAlertsOnTop(alertsOnTop: boolean) {
    setOptions({ alertsOnTop })
  }

  return (
    <OverviewSidebarOptionsRoot>
      <OverviewSidebarOptionsButtonsRoot>
        <AlertsOnTopToggle
          className={options.alertsOnTop ? "is-enabled" : ""}
          onClick={(_e) => setAlertsOnTop(!options.alertsOnTop)}
          analyticsName="ui.web.alertsOnTopToggle"
        >
          Alerts on Top
        </AlertsOnTopToggle>
      </OverviewSidebarOptionsButtonsRoot>
      <ResourceNameFilter />
    </OverviewSidebarOptionsRoot>
  )
}
