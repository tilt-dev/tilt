import React from "react"
import "./LoadingScreen.scss"

type props = {
  message?: string | React.ReactElement
}

function LoadingScreen(props: props) {
  let message = props.message || "Loading…"
  return <div className="LoadingScreen">{message}</div>
}

export default LoadingScreen
