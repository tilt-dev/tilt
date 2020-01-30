import React from "react"
import ReactDOM from "react-dom"
import "./index.scss"
import HUD from "./HUD"
import { Router } from "react-router-dom"
import { createBrowserHistory } from "history"
import ReactModal from "react-modal"

ReactModal.setAppElement(document.body)

let history = createBrowserHistory()
let app = (
  <Router history={history}>
    <HUD history={history} />
  </Router>
)
let root = document.getElementById("root")
ReactDOM.render(app, root)
