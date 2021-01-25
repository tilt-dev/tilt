import React from "react"
import styled from "styled-components"
import { ResourceFilters } from "./types"

const SidebarResourceTypeFilterStyle = styled.div`
  display: none;

  .isOverview & {
    display: flex;
  }
`

function handleCheckBoxClick(
  setter: (b: boolean) => void,
  event: React.ChangeEvent<HTMLInputElement>
) {
  setter(event.target.checked)
}

type SidebarResourceTypeFilterProps = {
  curState: ResourceFilters
  toggleShowServices: () => void
  toggleShowTests: () => void
}

export function SidebarResourceFilter(
  props: SidebarResourceTypeFilterProps
): JSX.Element {
  return (
    <SidebarResourceTypeFilterStyle>
      <div>
        <input
          type="checkbox"
          id="services"
          name="services"
          checked={props.curState.showServices}
          onChange={(evt) => props.toggleShowServices()}
        />
        <label htmlFor="services">Services</label>
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
    </SidebarResourceTypeFilterStyle>
  )
}
