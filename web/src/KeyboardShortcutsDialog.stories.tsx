import React from "react"
import { MemoryRouter } from "react-router"
import KeyboardShortcutsDialog from "./KeyboardShortcutsDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "New UI/Shared/KeyboardShortcutsDialog",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <Story />
      </MemoryRouter>
    ),
  ],
}

export const DialogOverview = () => (
  <KeyboardShortcutsDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
  />
)
