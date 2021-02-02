import Checkbox from "@material-ui/core/Checkbox"
import FormControlLabel from "@material-ui/core/FormControlLabel"
import { makeStyles } from "@material-ui/core/styles"
import CheckBoxIcon from "@material-ui/icons/CheckBox"
import CheckBoxOutlineBlankIcon from "@material-ui/icons/CheckBoxOutlineBlank"
import React from "react"
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

const FilterOptionList = styled.ul`
  ${mixinResetListStyle}
  display: flex;
  user-select: none; // Prevent unsightly highlighting on the label
`

const useStyles = makeStyles({
  root: {
    color: Color.offWhite,
  },
})

const AlertsOnTopToggle = styled.button`
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
  curState: SidebarOptions
  toggleShowResources: () => void
  toggleShowTests: () => void
  toggleAlertsOnTop: () => void
}

export function OverviewSidebarOptions(
  props: OverviewSidebarOptionsProps
): JSX.Element {
  const classes = useStyles()

  return (
    <OverviewSidebarOptionsRoot
      style={{ marginTop: SizeUnit(0.75), marginBottom: SizeUnit(-0.5) }}
    >
      <FilterOptionList>
        <FormControlLabel
          control={
            <Checkbox
              className={classes.root}
              color={"default"}
              icon={<CheckBoxOutlineBlankIcon fontSize="small" />}
              checkedIcon={<CheckBoxIcon fontSize="small" />}
              checked={props.curState.showResources}
              onChange={(e) => props.toggleShowResources()}
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
              checked={props.curState.showTests}
              onChange={(e) => props.toggleShowTests()}
              name="tests"
              id="tests"
            />
          }
          label="Tests"
        />
      </FilterOptionList>

      <AlertsOnTopToggle
        className={props.curState.alertsOnTop ? "is-enabled" : ""}
        onClick={(e) => props.toggleAlertsOnTop()}
      >
        Alerts on Top
      </AlertsOnTopToggle>
    </OverviewSidebarOptionsRoot>
  )
}
