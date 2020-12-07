import React from "react"
import SocketBar from "./SocketBar"
import { SocketState } from "./types"

export default {
  title: "SocketBar",
}

export const _Reconnecting = () => (
  <SocketBar state={SocketState.Reconnecting} />
)
