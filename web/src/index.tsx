import React from "react"
import ReactDOM from "react-dom"
import "./index.scss"
import HUD from "./HUD"
import NoMatch from "./NoMatch"
import { BrowserRouter as Router, Route, Switch } from "react-router-dom"

let app = <HUD />
let root = document.getElementById("root")
ReactDOM.render(app, root)
