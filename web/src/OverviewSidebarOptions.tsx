import { makeStyles } from "@material-ui/core/styles"
import React, { Dispatch, SetStateAction } from "react"
import styled from "styled-components"
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
  align-items: center;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallester};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  color: ${Color.offWhite};

  &.is-filtersHidden {
    justify-content: flex-end;
  }
`

export const FilterOptionList = styled.ul`
  ${mixinResetListStyle}
  display: flex;
  align-items: center;
  user-select: none; // Prevent unsightly highlighting on the label
`

const useStyles = makeStyles({
  root: {
    color: Color.offWhite,
  },
})

let testFilterControlsBorderRadius = "3px"

const ResourceFilterSegmentedControls = styled.div`
  margin-left: ${SizeUnit(0.25)};
`

const ResourceFilterSegmentedControl = styled.button`
  ${mixinResetButtonStyle}
  color: ${Color.grayLightest};
  background-color: ${Color.gray};
  padding: ${SizeUnit(0.125)} ${SizeUnit(0.25)};
  font-size: ${FontSize.smallester};

  &.is-enabled {
    color: ${Color.grayDarkest};
    background-color: ${Color.offWhite};
  }

  & + & {
    border-left: 2px solid ${Color.grayDark};
  }
`

export const TestsHiddenSegmentedControl = styled(
  ResourceFilterSegmentedControl
)`
  border-top-left-radius: ${testFilterControlsBorderRadius};
  border-bottom-left-radius: ${testFilterControlsBorderRadius};
`

export const TestsOnlySegmentedControl = styled(ResourceFilterSegmentedControl)`
  border-top-right-radius: ${testFilterControlsBorderRadius};
  border-bottom-right-radius: ${testFilterControlsBorderRadius};
`

export const AlertsOnTopToggle = styled.button`
  ${mixinResetButtonStyle}
  color: ${Color.grayLightest};
  background-color: ${Color.gray};
  padding: ${SizeUnit(0.125)} ${SizeUnit(0.25)};
  border-radius: 3px;
  font-size: ${FontSize.smallester};

  &.is-enabled {
    color: ${Color.grayDarkest};
    background-color: ${Color.offWhite};
  }
`

type OverviewSidebarOptionsProps = {
  showFilters: boolean
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

function toggleTestsOnly(props: OverviewSidebarOptionsProps) {
  props.setOptions((prevOptions) => {
    // Always set the option you're not currently toggling to 'false', because both
    // of these settings cannot be 'true' at the same time
    return {
      ...prevOptions,
      testsHidden: false,
      testsOnly: !prevOptions.testsOnly,
    }
  })
}

function toggleTestsHidden(props: OverviewSidebarOptionsProps) {
  props.setOptions((prevOptions) => {
    // Always set the option you're not currently toggling to 'false', because both
    // of these settings cannot be 'true' at the same time
    return {
      ...prevOptions,
      testsHidden: !prevOptions.testsHidden,
      testsOnly: false,
    }
  })
}

function filterOptions(props: OverviewSidebarOptionsProps) {
  const classes = useStyles()
  return (
    <FilterOptionList>
      Tests:
      <ResourceFilterSegmentedControls>
        <TestsHiddenSegmentedControl
          className={props.options.testsHidden ? "is-enabled" : ""}
          onClick={(e) => toggleTestsHidden(props)}
        >
          Hidden
        </TestsHiddenSegmentedControl>
        <TestsOnlySegmentedControl
          className={props.options.testsOnly ? "is-enabled" : ""}
          onClick={(e) => toggleTestsOnly(props)}
        >
          Only
        </TestsOnlySegmentedControl>
      </ResourceFilterSegmentedControls>
    </FilterOptionList>
  )
}

export function OverviewSidebarOptions(
  props: OverviewSidebarOptionsProps
): JSX.Element {
  return (
    <OverviewSidebarOptionsRoot
      style={{ marginTop: SizeUnit(0.75) }}
      className={!props.showFilters ? "is-filtersHidden" : ""}
    >
      {props.showFilters ? filterOptions(props) : null}
      <AlertsOnTopToggle
        className={props.options.alertsOnTop ? "is-enabled" : ""}
        onClick={(e) => setAlertsOnTop(props, !props.options.alertsOnTop)}
      >
        Alerts on Top
      </AlertsOnTopToggle>
    </OverviewSidebarOptionsRoot>
  )
}
