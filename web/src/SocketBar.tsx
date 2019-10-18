import React from "react"
import styled, { keyframes } from "styled-components"
import color from "./color"
import opacity from "./opacity"
import { SocketState } from "./types"

type SocketBarProps = {
  state: SocketState
}

let pulse = keyframes`
  0% {
    background-color: ${color.yellow};
  }
  50% {
    background-color: ${color.yellowLight};
  }
  100% {
    background-color: ${color.yellow};
  }
`

let Bar = styled.div`
  position: fixed;
  z-index: 1000;
  color: ${color.grayDarkest};
  background-color: ${color.yellow};
  width: 256px;
  margin-left: -128px;
  top: 128px;
  left: 50%;
  padding: 8px 16px;
  border-radius: 3px;
  box-shadow: -5px 5px 0 0 ${color.rgba(color.grayDarkest, opacity.obscured)};
  text-align: center;
  animation: ${pulse} 3s ease infinite;
`

export default function SocketBar(props: SocketBarProps) {
  let state = props.state
  let message = ""
  if (state === SocketState.Reconnecting) {
    message = "Reconnecting…"
  } else if (state === SocketState.Loading) {
    message = "Connecting…"
  }

  if (!message) {
    return null
  }
  return <Bar>{message}</Bar>
}
