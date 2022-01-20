import React from "react"
import { MemoryRouter } from "react-router"
import HelpDialog from "./HelpDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

export default {
  title: "New UI/Shared/HelpDialog",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <Story />
      </MemoryRouter>
    ),
  ],
}

export const DialogOverview = () => (
  <HelpDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
    isOverview={true}
  />
)
export const DialogLegacy = () => (
  <HelpDialog
    open={true}
    onClose={onRequestClose}
    anchorEl={document.body}
    isOverview={false}
  />
)
