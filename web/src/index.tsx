import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import { BrowserRouter } from "react-router-dom"
import { HUDFromContext } from "./HUD"
import "./index.scss"
import { InterfaceVersionProvider } from "./InterfaceVersion"
import { ThemeProvider } from "./ThemeContext"

ReactModal.setAppElement("#root")

let app = (
  <ThemeProvider>
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
  </ThemeProvider>
)
let root = document.getElementById("root")
ReactDOM.render(app, root)
