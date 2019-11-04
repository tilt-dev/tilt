import React from "react"
import { mount } from "enzyme"
import SidebarIcon, { IconType } from "./SidebarIcon"
import { RuntimeStatus, Build } from "./types"
import { Color } from "./constants"

type Ignore = boolean

const buildWithError = {
  error: {},
  startTime: "start time",
  log: "foobar",
  finishTime: "finish time",
  edits: ["foo.go"],
  isCrashRebuild: false,
  warnings: [],
}

const cases: Array<
  [
    string,
    RuntimeStatus,
    boolean,
    boolean,
    Color | Ignore,
    IconType | Ignore,
    boolean,
    Build | null
  ]
> = [
  [
    "auto mode, building with any status or warning state → small loader",
    RuntimeStatus.Pending,
    false,
    true,
    false,
    false,
    false,
    null,
  ],
  [
    "auto mode, status ok and no warning → small green dot",
    RuntimeStatus.Ok,
    false,
    false,
    Color.green,
    false,
    false,
    null,
  ],
  [
    "auto mode, status error and no warning → small red dot",
    RuntimeStatus.Error,
    false,
    false,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "auto mode, status error with warnings → small red dot",
    RuntimeStatus.Error,
    true,
    false,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "auto mode, status ok with warning → small yellow dot",
    RuntimeStatus.Ok,
    true,
    false,
    Color.yellow,
    false,
    false,
    null,
  ],
  [
    "auto mode, status pending and no warnings → small glowing ring",
    RuntimeStatus.Pending,
    false,
    false,
    false,
    IconType.StatusPending,
    false,
    null,
  ],
  [
    "auto mode, status pending with warnings → small glowing ring",
    RuntimeStatus.Pending,
    true,
    false,
    false,
    IconType.StatusPending,
    false,
    null,
  ],
]

test.each(cases)(
  "%s",
  (
    _,
    status,
    hasWarning,
    isBuilding,
    fillColor,
    iconType,
    isDirty,
    lastBuild
  ) => {
    const root = mount(
      <SidebarIcon
        status={status}
        hasWarning={hasWarning}
        isBuilding={isBuilding}
        isDirty={isDirty}
        lastBuild={lastBuild}
      />
    )

    if (fillColor !== false) {
      expect(root.find(`svg[fill="${fillColor}"]`)).toHaveLength(1)
    }

    if (iconType !== false) {
      let path = `svg.${iconType}`
      expect(root.find(path)).toHaveLength(1)
    }
  }
)
