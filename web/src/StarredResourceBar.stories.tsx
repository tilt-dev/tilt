import { Story } from "@storybook/react"
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

type StoryProps = {
  resources: ResourceNameAndStatus[]
  selectedResource?: string
}
const Template: Story<StoryProps> = (args) => (
  <StarredResourceBar
    resources={args.resources}
    unstar={(n) => {}}
    selectedResource={args.selectedResource}
  />
)

export const OneItem = Template.bind({})
OneItem.args = {
  resources: [{ name: "foo", status: ResourceStatus.Healthy }],
}

export const ThreeItems = Template.bind({})
ThreeItems.args = {
  resources: Array(3).fill({ name: "foo", status: ResourceStatus.Healthy }),
}

export const TwentyItems = Template.bind({})
TwentyItems.args = {
  resources: Array(20).fill({ name: "foobar", status: ResourceStatus.Healthy }),
}

export const LongName = Template.bind({})
LongName.args = {
  resources: [
    {
      name: "supercalifragilisticexpialidocious",
      status: ResourceStatus.Unhealthy,
    },
  ],
}

export const MixedNames = Template.bind({})
MixedNames.args = {
  resources: [
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
  selectedResource: "benchmark-muxer",
}

export const PendingActive = Template.bind({})
PendingActive.args = {
  resources: [{ name: "foo", status: ResourceStatus.Pending }],
  selectedResource: "foo",
}

export const BuildingActive = Template.bind({})
BuildingActive.args = {
  resources: [{ name: "foo", status: ResourceStatus.Building }],
  selectedResource: "foo",
}
