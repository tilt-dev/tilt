import React from "react"
import UpdateDialog from "./UpdateDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "UpdateDialog",
}

export const Dialog = () => (
  <UpdateDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
    showUpdate={true}
    suggestedVersion={"0.18.1"}
  />
)
