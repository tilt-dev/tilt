import React from "react"
import { mount } from "enzyme"
import SidebarIcon, { IconType } from "./SidebarIcon"
import { ResourceStatus, TriggerMode, Build } from "./types"
import { Color } from "./constants"

type Ignore = boolean

const buildWithError = {
  Error: {},
  StartTime: "start time",
  Log: "foobar",
  FinishTime: "finish time",
  Edits: ["foo.go"],
  IsCrashRebuild: false,
  Warnings: [],
}

const cases: Array<
  [
    string,
    ResourceStatus,
    boolean,
    boolean,
    TriggerMode,
    Color | Ignore,
    IconType | Ignore,
    boolean,
    Build | null
  ]
> = [
  [
    "auto mode, building with any status or warning state → small loader",
    ResourceStatus.Pending,
    false,
    true,
    TriggerMode.TriggerModeAuto,
    false,
    false,
    false,
    null,
  ],
  [
    "manual mode, building with any status or warning state → loader",
    ResourceStatus.Ok,
    false,
    true,
    TriggerMode.TriggerModeAuto,
    false,
    false,
    false,
    null,
  ],
  [
    "auto mode, status ok and no warning → small green dot",
    ResourceStatus.Ok,
    false,
    false,
    TriggerMode.TriggerModeAuto,
    Color.green,
    false,
    false,
    null,
  ],
  [
    "manual mode, status ok and no warning → green ring",
    ResourceStatus.Ok,
    false,
    false,
    TriggerMode.TriggerModeManual,
    Color.green,
    false,
    false,
    null,
  ],
  [
    "auto mode, status error and no warning → small red dot",
    ResourceStatus.Error,
    false,
    false,
    TriggerMode.TriggerModeAuto,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "manual mode, status error and no warning → red ring",
    ResourceStatus.Error,
    false,
    false,
    TriggerMode.TriggerModeManual,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "auto mode, status error with warnings → small red dot",
    ResourceStatus.Error,
    true,
    false,
    TriggerMode.TriggerModeAuto,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "manual mode, status error with warnings → red ring",
    ResourceStatus.Error,
    true,
    false,
    TriggerMode.TriggerModeManual,
    Color.red,
    false,
    false,
    null,
  ],
  [
    "auto mode, status ok with warning → small yellow dot",
    ResourceStatus.Ok,
    true,
    false,
    TriggerMode.TriggerModeAuto,
    Color.yellow,
    false,
    false,
    null,
  ],
  [
    "manual mode, status ok with warning → yellow ring",
    ResourceStatus.Ok,
    true,
    false,
    TriggerMode.TriggerModeManual,
    Color.yellow,
    false,
    false,
    null,
  ],
  [
    "auto mode, status pending and no warnings → small glowing ring",
    ResourceStatus.Pending,
    false,
    false,
    TriggerMode.TriggerModeAuto,
    false,
    IconType.DotAutoPending,
    false,
    null,
  ],
  [
    "auto mode, status pending with warnings → small glowing ring",
    ResourceStatus.Pending,
    true,
    false,
    TriggerMode.TriggerModeAuto,
    false,
    IconType.DotAutoPending,
    false,
    null,
  ],
  [
    "manual mode, status pending and no warnings → glowing ring",
    ResourceStatus.Pending,
    false,
    false,
    TriggerMode.TriggerModeManual,
    Color.gray,
    IconType.DotManualPending,
    false,
    null,
  ],
  [
    "manual mode, status pending with warnings → glowing ring",
    ResourceStatus.Pending,
    true,
    false,
    TriggerMode.TriggerModeManual,
    Color.gray,
    IconType.DotManualPending,
    false,
    null,
  ],
  [
    "manual mode, status pending with last build in error → red ring",
    ResourceStatus.Pending,
    false,
    false,
    TriggerMode.TriggerModeManual,
    Color.red,
    IconType.DotManual,
    true,
    buildWithError,
  ],
]

test.each(cases)(
  "%s",
  (
    _,
    status,
    hasWarning,
    isBuilding,
    triggerMode,
    fillColor,
    iconType,
    isDirty,
    lastBuild
  ) => {
    const root = mount(
      <SidebarIcon
        status={status}
        hasWarning={hasWarning}
        triggerMode={triggerMode}
        isBuilding={isBuilding}
        isDirty={isDirty}
        lastBuild={lastBuild}
      />
    )

    const triggerClass =
      triggerMode === TriggerMode.TriggerModeAuto ? "svg.auto" : "svg.manual"
    expect(root.find(triggerClass)).toHaveLength(1)

    if (fillColor !== false) {
      expect(root.find(`svg[fill="${fillColor}"]`)).toHaveLength(1)
    }

    if (iconType !== false) {
      let path = `svg.${iconType}`
      expect(root.find(path)).toHaveLength(1)
    }
  }
)
