import React from "react"
import styled, { keyframes } from "styled-components"
import { Color, ColorAlpha, ColorRGBA, ZIndex } from "./style-helpers"
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

let SocketBarRoot = styled.div`
  position: fixed;
  z-index: ${ZIndex.SocketBar};
  width: 100vw;
  display: flex;
  top: 0;
  left: 0;
`

let Bar = styled.div`
  color: ${Color.gray10};
  background-color: ${Color.yellow};
  margin: auto;
  margin-top: 64px;
  padding: 8px 16px;
  border-radius: 3px;
  box-shadow: -5px 5px 0 0 ${ColorRGBA(Color.gray10, ColorAlpha.almostOpaque)};
  text-align: center;
  animation: ${pulse} 3s ease infinite;
`

export function isTiltSocketConnected(state: SocketState) {
  if (
    state === SocketState.Reconnecting ||
    state === SocketState.Closed ||
    state === SocketState.Loading
  ) {
    return false
  }

  return true
}

export default function SocketBar(props: SocketBarProps) {
  let state = props.state
  let message = ""
  if (state === SocketState.Reconnecting) {
    message = "Connection failed: reconnecting…"
  } else if (state === SocketState.Loading) {
    message = "Connecting…"
  }

  if (!message) {
    return null
  }
  return (
    <SocketBarRoot>
      <Bar>{message}</Bar>
    </SocketBarRoot>
  )
}
