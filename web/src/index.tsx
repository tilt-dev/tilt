import React from "react"
import { createRoot } from "react-dom/client"
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
let root = createRoot(document.getElementById("root")!)
root.render(app)
