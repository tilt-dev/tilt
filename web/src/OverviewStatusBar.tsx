import React from "react"
import styled from "styled-components"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

type OverviewStatusBarProps = {
  build?: Proto.webviewTiltBuild
}

let OverviewStatusBarRoot = styled.div`
  display: flex;
  width: 100%;
  background-color: ${Color.white};
  align-items: center;
  justify-content: flex-end;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
  color: ${Color.gray};
  padding: 0 ${SizeUnit(0.75)};
  font-weight: 400;
  box-sizing: border-box;
  flex: 0 0 ${SizeUnit(1)};
`

export default function OverviewStatusBar(props: OverviewStatusBarProps) {
  let version = props.build?.version || "?.?.?"
  let text = `Tilt v${version}`
  return (
    <OverviewStatusBarRoot>
      <div>{text}</div>
    </OverviewStatusBarRoot>
  )
}
