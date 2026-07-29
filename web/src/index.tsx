import React from "react"
import { createRoot } from "react-dom/client"
import ReactModal from "react-modal"
import { BrowserRouter } from "react-router-dom"
import { HUDFromContext } from "./HUD"
import "./index.scss"
import { InterfaceVersionProvider } from "./InterfaceVersion"

ReactModal.setAppElement("#root")

let app = (
  <BrowserRouter
    future={{
      v7_relativeSplatPath: true,
      v7_startTransition: true,
    }}
  >
    <InterfaceVersionProvider>
      <HUDFromContext />
    </InterfaceVersionProvider>
  </BrowserRouter>
)
let root = createRoot(document.getElementById("root")!)
root.render(app)
