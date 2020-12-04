import { storiesOf } from "@storybook/react"
import React from "react"
import FatalErrorModal from "./FatalErrorModal"
import { ShowFatalErrorModal } from "./types"

const fakeOnClose = () => {}
const longError = "ERROR\n".repeat(500)

storiesOf("FatalErrorModal", module)
  .add("shortError", () => (
    <FatalErrorModal
      error="this is an error"
      handleClose={fakeOnClose}
      showFatalErrorModal={ShowFatalErrorModal.Default}
    ></FatalErrorModal>
  ))
  .add("longError", () => (
    <FatalErrorModal
      error={longError}
      handleClose={fakeOnClose}
      showFatalErrorModal={ShowFatalErrorModal.Default}
    ></FatalErrorModal>
  ))
