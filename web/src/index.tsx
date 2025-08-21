import React from "react"
import ReactDOM from "react-dom"
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
let root = document.getElementById("root")
ReactDOM.render(app, root)
