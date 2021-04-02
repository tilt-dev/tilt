import React from "react"
import StarredResourceBar, { ResourceNameAndStatus } from "./StarredResourceBar"
import { Color } from "./style-helpers"
import { ResourceStatus } from "./types"

export default {
  title: "New UI/Shared/StarredResourceBar",
  decorators: [
    // make the pane bg black so that the menu bg stands out
    (Story: any) => (
      <div style={{ backgroundColor: Color.black, height: "300px" }}>
        <Story />
      </div>
    ),
  ],
}

function story(resources: ResourceNameAndStatus[], selectedResource?: string) {
  return (
    <StarredResourceBar
      resources={resources}
      unstar={(n) => {}}
      selectedResource={selectedResource}
    />
  )
}

export const OneItem = () =>
  story([{ name: "foo", status: ResourceStatus.Healthy }])

export const ThreeItems = () =>
  story(Array(3).fill(["foo", ResourceStatus.Healthy]))

export const TwentyItems = () =>
  story(Array(20).fill(["foobar", ResourceStatus.Healthy]))

export const LongName = () =>
  story([
    {
      name: "supercalifragilisticexpialidocious",
      status: ResourceStatus.Unhealthy,
    },
  ])

export const MixedNames = () =>
  story(
    [
      { name: "max-object-detected-name", status: ResourceStatus.Healthy },
      { name: "muxer", status: ResourceStatus.Unhealthy },
      { name: "benchmark-muxer", status: ResourceStatus.Healthy },
      { name: "benchmark-all", status: ResourceStatus.Healthy },
      { name: "recompile-storage", status: ResourceStatus.Unhealthy },
      { name: "benchamrk-rectangle-test", status: ResourceStatus.Healthy },
      { name: "benchmark-storage", status: ResourceStatus.Healthy },
      { name: "(Tiltfile)", status: ResourceStatus.Pending },
      { name: "recompile-rectangle-test", status: ResourceStatus.Unhealthy },
      { name: "SillyOne test", status: ResourceStatus.Unhealthy },
      { name: "AttackOfTheSilly test", status: ResourceStatus.Unhealthy },
    ],
    "benchmark-muxer"
  )
