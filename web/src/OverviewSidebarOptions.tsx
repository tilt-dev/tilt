import React, { Dispatch, SetStateAction } from "react"
import styled from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import { ResourceNameFilter } from "./ResourceNameFilter"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  mixinResetListStyle,
  SizeUnit,
} from "./style-helpers"
import { SidebarOptions } from "./types"

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
  font-size: ${FontSize.smallester};

  &.is-enabled {
    color: ${Color.grayDarkest};
    background-color: ${Color.offWhite};
  }
`

type OverviewSidebarOptionsProps = {
  options: SidebarOptions
  setOptions: Dispatch<SetStateAction<SidebarOptions>>
}

function setAlertsOnTop(
  props: OverviewSidebarOptionsProps,
  alertsOnTop: boolean
) {
  props.setOptions((prevOptions) => {
    return { ...prevOptions, alertsOnTop: alertsOnTop }
  })
}

export function OverviewSidebarOptions(
  props: OverviewSidebarOptionsProps
): JSX.Element {
  return (
    <OverviewSidebarOptionsRoot>
      <OverviewSidebarOptionsButtonsRoot>
        <AlertsOnTopToggle
          className={props.options.alertsOnTop ? "is-enabled" : ""}
          onClick={(_e) => setAlertsOnTop(props, !props.options.alertsOnTop)}
          analyticsName="ui.web.alertsOnTopToggle"
        >
          Alerts on Top
        </AlertsOnTopToggle>
      </OverviewSidebarOptionsButtonsRoot>
      <ResourceNameFilter />
    </OverviewSidebarOptionsRoot>
  )
}
