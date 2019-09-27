import React from "react"
import { storiesOf } from "@storybook/react"

import ShareSnapshotModal from "./ShareSnapshotModal"

let handleSendSnapshot = () => console.log("sendSnapshot")
let handleClose = () => console.log("close")

let signedOut = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={""}
      tiltCloudSchemeHost={"https://cloud.tilt.dev/"}
      isOpen={true}
    />
  )
}
let signedIn = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev/"}
      isOpen={true}
    />
  )
}

let withUrl = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={"https://cloud.tilt.dev/snapshot/rose-quartz"}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev/"}
      isOpen={true}
    />
  )
}

storiesOf("ShareSnapshotModal", module)
  .add("signed-out", signedOut)
  .add("signed-in", signedIn)
  .add("with-url", withUrl)
