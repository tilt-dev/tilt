import React from "react"
import Tooltip from "./Tooltip"

export default {
  title: "Tooltip",
}

export const Default = () => (
  <div
    style={{
      display: "flex",
      width: "500px",
      height: "500px",
      alignItems: "center",
      justifyContent: "center",
    }}
  >
    <Tooltip title="icon explanation" open={true}>
      <span>Hello world</span>
    </Tooltip>
  </div>
)
