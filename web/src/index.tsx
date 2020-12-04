import { createBrowserHistory } from "history"
import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import { Router } from "react-router-dom"
import HUD from "./HUD"
import "./index.scss"

ReactModal.setAppElement(document.body)

let history = createBrowserHistory()
let app = (
  <Router history={history}>
    <HUD history={history} />
  </Router>
)
let root = document.getElementById("root")
ReactDOM.render(app, root)
