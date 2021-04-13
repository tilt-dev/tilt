import { debounce, InputAdornment, TextField } from "@material-ui/core"
import { InputProps as StandardInputProps } from "@material-ui/core/Input/Input"
import React, { Dispatch, SetStateAction } from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as SearchSvg } from "./assets/svg/search.svg"
import { InstrumentedButton } from "./instrumentedComponents"
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
  &.is-filterButtonsHidden {
    justify-content: flex-end;
  }
`

export const FilterOptionList = styled.ul`
  ${mixinResetListStyle};
  display: flex;
  align-items: center;
  user-select: none; // Prevent unsightly highlighting on the label
`

const toggleBorderRadius = "3px"

const ResourceFilterSegmentedControls = styled.div`
  margin-left: ${SizeUnit(0.25)};
`

const ResourceFilterToggle = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
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

export const TestsHiddenToggle = styled(ResourceFilterToggle)`
  border-top-left-radius: ${toggleBorderRadius};
  border-bottom-left-radius: ${toggleBorderRadius};
`

export const TestsOnlyToggle = styled(ResourceFilterToggle)`
  border-top-right-radius: ${toggleBorderRadius};
  border-bottom-right-radius: ${toggleBorderRadius};
`

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

export const ResourceNameFilterTextField = styled(TextField)`
  & .MuiOutlinedInput-root {
    border-radius: ${SizeUnit(0.5)};
    border: 1px solid ${Color.grayLighter};
    background-color: ${Color.gray};

    & fieldset {
      border-color: 1px solid ${Color.grayLighter};
    }
    &:hover fieldset {
      border: 1px solid ${Color.grayLighter};
    }
    &.Mui-focused fieldset {
      border: 1px solid ${Color.grayLighter};
    }
    & .MuiOutlinedInput-input {
      padding: ${SizeUnit(0.2)};
    }
  }

  margin-top: ${SizeUnit(0.4)};
  margin-bottom: ${SizeUnit(0.4)};

  & .MuiInputBase-input {
    font-family: ${Font.monospace};
    color: ${Color.offWhite};
    font-size: ${FontSize.small};
  }
`

export const ClearResourceNameFilterButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
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

function setResourceNameFilter(
  newValue: string,
  props: OverviewSidebarOptionsProps
) {
  props.setOptions((prevOptions) => {
    return {
      ...prevOptions,
      resourceNameFilter: newValue,
    }
  })
}

function filterOptions(props: OverviewSidebarOptionsProps) {
  return (
    <FilterOptionList>
      Tests:
      <ResourceFilterSegmentedControls>
        <TestsHiddenToggle
          className={props.options.testsHidden ? "is-enabled" : ""}
          onClick={(e) => toggleTestsHidden(props)}
          analyticsName="ui.web.testsHiddenToggle"
          analyticsTags={{
            newTestsHiddenState: (!props.options.testsHidden).toString(),
          }}
        >
          Hidden
        </TestsHiddenToggle>
        <TestsOnlyToggle
          className={props.options.testsOnly ? "is-enabled" : ""}
          onClick={(e) => toggleTestsOnly(props)}
          analyticsName="ui.web.testsOnlyToggle"
          analyticsTags={{
            newTestsOnlyState: (!props.options.testsOnly).toString(),
          }}
        >
          Only
        </TestsOnlyToggle>
      </ResourceFilterSegmentedControls>
    </FilterOptionList>
  )
}

// debounce so we don't send for every single keypress
let incrResourceNameFilterEdit = debounce(() => {
  incr("ui.web.resourceNameFilter", { action: "edit" })
}, 5000)

function ResourceNameFilter(props: OverviewSidebarOptionsProps) {
  let inputProps: Partial<StandardInputProps> = {
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={Color.grayLightest} />
      </InputAdornment>
    ),
  }

  // only show the "x" to clear if there's any input to clear
  if (props.options.resourceNameFilter) {
    const onClearClick = () => {
      setResourceNameFilter("", props)
    }

    inputProps.endAdornment = (
      <InputAdornment position="end">
        <ClearResourceNameFilterButton
          onClick={onClearClick}
          analyticsName="ui.web.clearResourceNameFilter"
        >
          <CloseSvg fill={Color.grayLightest} />
        </ClearResourceNameFilterButton>
      </InputAdornment>
    )
  }

  const onChange = (e: any) => {
    incrResourceNameFilterEdit()
    setResourceNameFilter(e.target.value, props)
  }

  return (
    <ResourceNameFilterTextField
      value={props.options.resourceNameFilter ?? ""}
      onChange={onChange}
      placeholder="Filter resources by name"
      InputProps={inputProps}
      variant="outlined"
    />
  )
}

export function OverviewSidebarOptions(
  props: OverviewSidebarOptionsProps
): JSX.Element {
  return (
    <OverviewSidebarOptionsRoot>
      <OverviewSidebarOptionsButtonsRoot
        className={!props.showFilters ? "is-filterButtonsHidden" : ""}
      >
        {props.showFilters ? filterOptions(props) : null}
        <AlertsOnTopToggle
          className={props.options.alertsOnTop ? "is-enabled" : ""}
          onClick={(e) => setAlertsOnTop(props, !props.options.alertsOnTop)}
          analyticsName="ui.web.alertsOnTopToggle"
        >
          Alerts on Top
        </AlertsOnTopToggle>
      </OverviewSidebarOptionsButtonsRoot>
      <ResourceNameFilter {...props} />
    </OverviewSidebarOptionsRoot>
  )
}
