import React from "react"
import ShortcutsDialog from "./ShortcutsDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "ShortcutsDialog",
}

export const Dialog = () => (
  <ShortcutsDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
  />
)
