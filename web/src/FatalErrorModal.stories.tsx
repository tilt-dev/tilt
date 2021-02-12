import React from "react"
import FatalErrorModal from "./FatalErrorModal"
import { ShowFatalErrorModal } from "./types"

const fakeOnClose = () => {}
const longError = "ERROR\n".repeat(500)

export default {
  title: "New UI/_To Review/FatalErrorModal",
}

export const ShortError = () => (
  <FatalErrorModal
    error="this is an error"
    handleClose={fakeOnClose}
    showFatalErrorModal={ShowFatalErrorModal.Default}
  ></FatalErrorModal>
)

export const LongError = () => (
  <FatalErrorModal
    error={longError}
    handleClose={fakeOnClose}
    showFatalErrorModal={ShowFatalErrorModal.Default}
  ></FatalErrorModal>
)
