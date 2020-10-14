import React from "react"
import { storiesOf } from "@storybook/react"
import ShortcutsDialog from "./ShortcutsDialog"

function onRequestClose() {
  console.log("onRequestClose")
}

storiesOf("ShortcutsDialog", module).add("dialog", () => (
  <ShortcutsDialog isOpen={true} onRequestClose={onRequestClose} />
))
