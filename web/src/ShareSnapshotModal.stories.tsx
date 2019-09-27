import React from "react"
import { storiesOf } from "@storybook/react"

import ShareSnapshotModal from "./ShareSnapshotModal"
import ReactModal from "react-modal"

ReactModal.setAppElement(document.body)

let handleSendSnapshot = () => console.log("sendSnapshot")
let handleClose = () => console.log("close")

let signedOut = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={""}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
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
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      isOpen={true}
    />
  )
}

let withUrl = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={"https://cloud.tilt.dev/snapshot/garnet"}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      isOpen={true}
    />
  )
}

let withUrlOverflow = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={
        "https://cloud.tilt.dev/snapshot/rose-quartz-long-overflow-string"
      }
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      isOpen={true}
    />
  )
}

storiesOf("ShareSnapshotModal", module)
  .add("signed-out", signedOut)
  .add("signed-in", signedIn)
  .add("with-url", withUrl)
  .add("with-url-overflow", withUrlOverflow)
