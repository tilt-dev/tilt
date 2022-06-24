import React from "react"
import ReactModal from "react-modal"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { nResourceView } from "./testdata"

ReactModal.setAppElement("#root")

let handleClose = () => console.log("close")

const offlineSnapshot = () => {
  const anchorEl: HTMLElement | null = document.body.querySelector("#root")
  return (
    <ShareSnapshotModal
      getSnapshot={() => ({ view: nResourceView(1) })}
      handleClose={handleClose}
      isOpen={true}
      dialogAnchor={anchorEl}
    />
  )
}

export default {
  title: "New UI/Shared/ShareSnapshotModal",
}

export const OfflineSnapshot = offlineSnapshot
