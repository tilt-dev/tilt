import React from "react"
import styled, { keyframes } from "styled-components"
import { Color, ColorAlpha, ColorRGBA } from "./style-helpers"
import { SocketState } from "./types"

type SocketBarProps = {
  state: SocketState
}

let pulse = keyframes`
  0% {
    background-color: ${Color.yellow};
  }
  50% {
    background-color: ${Color.yellowLight};
  }
  100% {
    background-color: ${Color.yellow};
  }
`

let Bar = styled.div`
  position: fixed;
  z-index: 1000;
  color: ${Color.grayDarkest};
  background-color: ${Color.yellow};
  width: 256px;
  margin-left: -128px;
  top: 128px;
  left: 50%;
  padding: 8px 16px;
  border-radius: 3px;
  box-shadow: -5px 5px 0 0
    ${ColorRGBA(Color.grayDarkest, ColorAlpha.almostOpaque)};
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
