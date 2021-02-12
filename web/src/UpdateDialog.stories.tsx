import React from "react"
import { FakeInterfaceVersionProvider } from "./InterfaceVersion"
import UpdateDialog from "./UpdateDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "Legacy UI/UpdateDialog",
}

export const Dialog = () => (
  <FakeInterfaceVersionProvider>
    <UpdateDialog
      open={true}
      onClose={onRequestClose}
      anchorEl={document.body}
      showUpdate={true}
      suggestedVersion={"0.18.1"}
      isNewInterface={false}
    />
  </FakeInterfaceVersionProvider>
)

export const DialogNoUpdate = () => (
  <FakeInterfaceVersionProvider>
    <UpdateDialog
      open={true}
      onClose={onRequestClose}
      anchorEl={document.body}
      showUpdate={false}
      suggestedVersion={"0.18.1"}
      isNewInterface={false}
    />
  </FakeInterfaceVersionProvider>
)
