import React from "react"
import HeroScreen from "./HeroScreen"
import SocketBar from "./SocketBar"
import { SocketState } from "./types"

export default {
  title: "New UI/HeroScreen",
}

export const Loading = () => <HeroScreen>Loading…</HeroScreen>

export const WithSocketBar = () => (
  <HeroScreen>
    <SocketBar state={SocketState.Reconnecting} />
    Loading…
  </HeroScreen>
)
