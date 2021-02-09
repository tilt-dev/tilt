import Checkbox from "@material-ui/core/Checkbox"
import FormControlLabel from "@material-ui/core/FormControlLabel"
import { makeStyles } from "@material-ui/core/styles"
import CheckBoxIcon from "@material-ui/icons/CheckBox"
import CheckBoxOutlineBlankIcon from "@material-ui/icons/CheckBoxOutlineBlank"
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
`

export const FilterOptionList = styled.ul<{ visible: boolean }>`
  ${mixinResetListStyle}
  display: flex;
  user-select: none; // Prevent unsightly highlighting on the label
  visibility: ${(props) => (props.visible ? "default" : "hidden")};
`

const useStyles = makeStyles({
  root: {
    color: Color.offWhite,
  },
})

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

function setShowTests(props: OverviewSidebarOptionsProps, showTests: boolean) {
  props.setOptions((prevOptions) => {
    return { ...prevOptions, showTests: showTests }
  })
}

function setShowResources(
  props: OverviewSidebarOptionsProps,
  showResources: boolean
) {
  props.setOptions((prevOptions) => {
    return { ...prevOptions, showResources: showResources }
  })
}

export function OverviewSidebarOptions(
  props: OverviewSidebarOptionsProps
): JSX.Element {
  const classes = useStyles()

  return (
    <OverviewSidebarOptionsRoot
      style={{ marginTop: SizeUnit(0.75), marginBottom: SizeUnit(-0.5) }}
    >
      <FilterOptionList visible={props.showFilters}>
        <FormControlLabel
          control={
            <Checkbox
              className={classes.root}
              color={"default"}
              icon={<CheckBoxOutlineBlankIcon fontSize="small" />}
              checkedIcon={<CheckBoxIcon fontSize="small" />}
              checked={props.options.showResources}
              onChange={(e) => setShowResources(props, e.target.checked)}
              name="resources"
              id="resources"
            />
          }
          label="Resources"
        />
        <FormControlLabel
          control={
            <Checkbox
              className={classes.root}
              color={"default"}
              icon={<CheckBoxOutlineBlankIcon fontSize="small" />}
              checkedIcon={<CheckBoxIcon fontSize="small" />}
              checked={props.options.showTests}
              onChange={(e) => setShowTests(props, e.target.checked)}
              name="tests"
              id="tests"
            />
          }
          label="Tests"
        />
      </FilterOptionList>

      <AlertsOnTopToggle
        className={props.options.alertsOnTop ? "is-enabled" : ""}
        onClick={(e) => setAlertsOnTop(props, !props.options.alertsOnTop)}
      >
        Alerts on Top
      </AlertsOnTopToggle>
    </OverviewSidebarOptionsRoot>
  )
}
