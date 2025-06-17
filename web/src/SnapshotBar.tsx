import moment from "moment"
import React from "react"
import styled from "styled-components"
import { usePathBuilder } from "./PathBuilder"
import { useSnapshotAction } from "./snapshot"
import { Color, FontSize, SizeUnit } from "./style-helpers"

const SnapshotBanner = styled.div`
  background-color: ${Color.offWhite};
  box-sizing: border-box;
  color: ${Color.black};
  font-size: ${FontSize.small};
  height: ${SizeUnit(1)};
  padding: 3px 10px;
  width: 100%;

  /* There's a small layout shift in the header
  bar on Detail View because of the scrollbar,
  so offset it on Table View */
  &.is-grid {
    margin-bottom: -2px;
  }
`

const SnapshotTitle = styled.span`
  font-weight: bold;
  text-decoration: underline;
`

export function SnapshotBar(props: { className?: string }) {
  const pb = usePathBuilder()
  const { currentSnapshotTime } = useSnapshotAction()

  const isSnapshot = pb.isSnapshot()
  if (!isSnapshot) {
    return null
  }

  let timestampDescription = ""
  if (currentSnapshotTime?.createdAt) {
    const createdAt = moment(currentSnapshotTime?.createdAt).format("lll")
    timestampDescription = `(created at ${createdAt})`
  } else if (currentSnapshotTime?.tiltUpTime) {
    const tiltUpTime = moment(currentSnapshotTime?.tiltUpTime).format("lll")
    timestampDescription = `(session started at ${tiltUpTime})`
  }

  return (
    <SnapshotBanner role="status" className={props.className}>
      <SnapshotTitle>Snapshot</SnapshotTitle> {timestampDescription}
    </SnapshotBanner>
  )
}
