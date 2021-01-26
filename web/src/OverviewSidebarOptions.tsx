import React from "react"
import styled from "styled-components"
import { SidebarOptions } from "./types"

const SidebarOptionsSetterStyle = styled.div`
  display: flex;
`

type SidebarResourceTypeFilterProps = {
  curState: SidebarOptions
  toggleShowResources: () => void
  toggleShowTests: () => void
}

export function SidebarOptionsSetter(
  props: SidebarResourceTypeFilterProps
): JSX.Element {
  return (
    <SidebarOptionsSetterStyle>
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
    </SidebarOptionsSetterStyle>
  )
}
