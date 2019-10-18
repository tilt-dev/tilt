import React from "react"
import { storiesOf } from "@storybook/react"
import { SocketState } from "./types"
import SocketBar from "./SocketBar"

storiesOf("SocketBar", module).add("reconnecting", () => (
  <SocketBar state={SocketState.Reconnecting} />
))
