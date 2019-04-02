import React from "react"
import { ReactComponent as LogoSvg } from "./assets/svg/logo.svg"
import "./LogoButton.scss"

type props = {
  onclick: () => void
}

const LogoButton = React.memo((props: props) => {
  return (
    <div>
      <button className="LogoButton" onClick={props.onclick}>
        <LogoSvg />
      </button>
    </div>
  )
})

export default LogoButton
