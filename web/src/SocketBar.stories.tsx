import { storiesOf } from "@storybook/react"
import React from "react"
import SocketBar from "./SocketBar"
import { SocketState } from "./types"

storiesOf("SocketBar", module).add("reconnecting", () => (
  <SocketBar state={SocketState.Reconnecting} />
))
