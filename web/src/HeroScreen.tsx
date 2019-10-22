import React from "react"
import "./HeroScreen.scss"

type props = {
  message?: string | React.ReactElement
}

function HeroScreen(props: props) {
  let message = props.message || "Loading…"
  return <section className="HeroScreen">{message}</section>
}

export default HeroScreen
