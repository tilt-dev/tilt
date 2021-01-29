import React from "react"
import styled from "styled-components"
import { SidebarOptions } from "./types"

const OverviewSidebarOptionsRoot = styled.div`
  display: flex;
`

// Crappy placeholder styles so these things look distinct while I'm coding
// them -- HAN FIX MEEEEE 😭
const FilterOptions = styled.div`
  float: left;
`
const SortOptions = styled.div`
  float: right;
  padding-left: 2.5em;
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
  return (
    <OverviewSidebarOptionsRoot>
      <FilterOptions>
        <div>
          <input
            type="checkbox"
            id="resources"
            name="resources"
            checked={props.curState.showResources}
            onChange={(evt) => props.toggleShowResources()}
          />
          <label htmlFor="resources">Resources</label>
        </div>
        <div>
          <input
            type="checkbox"
            id="tests"
            name="tests"
            checked={props.curState.showTests}
            onChange={(evt) => props.toggleShowTests()}
          />
          <label htmlFor="tests">Tests</label>
        </div>
      </FilterOptions>

      <SortOptions>
        <div>
          <input
            type="checkbox"
            id="alertsOnTop"
            name="alertsOnTop"
            checked={props.curState.alertsOnTop}
            onChange={(evt) => props.toggleAlertsOnTop()}
          />
          <label htmlFor="alertsontop">Alerts On Top</label>
        </div>
      </SortOptions>
    </OverviewSidebarOptionsRoot>
  )
}
