import React from "react"
import { MemoryRouter } from "react-router"
import ShortcutsDialog from "./ShortcutsDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "New UI/Shared/ShortcutsDialog",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <Story />
      </MemoryRouter>
    ),
  ],
}

export const DialogOverview = () => (
  <ShortcutsDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
    isOverview={true}
  />
)
export const DialogLegacy = () => (
  <ShortcutsDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
    isOverview={false}
  />
)
