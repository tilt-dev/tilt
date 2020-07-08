import React from "react"
import ReactDOM from "react-dom"
import "./index.scss"
import HUD from "./HUD"
import { Router } from "react-router-dom"
import { createBrowserHistory } from "history"
import ReactModal from "react-modal"
import { incr } from "./analytics"

ReactModal.setAppElement(document.body)

let history = createBrowserHistory()
let app = (
  <Router history={history}>
    <HUD history={history} incr={incr} />
  </Router>
)
let root = document.getElementById("root")
ReactDOM.render(app, root)
