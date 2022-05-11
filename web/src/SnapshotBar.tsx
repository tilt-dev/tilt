import React from "react"
import styled from "styled-components"
import { Color, FontSize, SizeUnit, ZIndex } from "./style-helpers"

type SnapshotBarProps = {
  isSnapshot: boolean
  snapshotTime: string | undefined
  tiltUpTime: string | undefined
}

const SnapshotBanner = styled.div`
  background-color: ${Color.offWhite};
  box-sizing: border-box;
  color: ${Color.black};
  font-size: ${FontSize.small};
  height: ${SizeUnit(1)};
  padding: 3px 10px;
  position: absolute;
  top: 0;
  width: 100%;
  z-index: ${ZIndex.SnapshotBar};
`

const SnapshotTitle = styled.span`
  font-weight: bold;
  text-decoration: underline;
`

export function SnapshotBar(props: SnapshotBarProps) {
  if (!props.isSnapshot) {
    return null
  }

  let timestampDescription = ""
  if (props.snapshotTime) {
    timestampDescription = `(created at ${props.snapshotTime})` // TODO: Formatting here
  } else if (props.tiltUpTime) {
    timestampDescription = `(session started at ${props.tiltUpTime})`
  }

  return (
    <SnapshotBanner role="status">
      <SnapshotTitle>Snapshot</SnapshotTitle> {timestampDescription}
    </SnapshotBanner>
  )
}
