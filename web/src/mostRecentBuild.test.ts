import mostRecentBuildToDisplay, { ResourceWithBuilds } from "./mostRecentBuild"
import { zeroTime } from "./time"

it("returns null if there are no builds", () => {
  const resources: ResourceWithBuilds[] = []

  let actual = mostRecentBuildToDisplay(resources)
  expect(actual).toBeNull()
})

it("returns the most recent build if there are no pending builds", () => {
  let recent = {
    edits: ["main.go"],
    startTime: "2019-04-24T13:08:41.017623-04:00",
    finishTime: "2019-04-24T13:08:42.926608-04:00",
    isCrashRebuild: false,
    warnings: [],
  }
  let expectedTuple = {
    name: "snack",
    edits: ["main.go"],
    since: recent.startTime,
  }
  const resource: ResourceWithBuilds = {
    name: "snack",
    buildHistory: [
      {
        edits: ["main.go"],
        startTime: "2019-04-24T13:08:39.017623-04:00",
        finishTime: "2019-04-24T13:08:40.926608-04:00",
        isCrashRebuild: false,
        warnings: [],
      },
      recent,
    ],
    pendingBuildEdits: null,
    pendingBuildSince: zeroTime,
  }
  const resources: ResourceWithBuilds[] = [resource]

  let actual = mostRecentBuildToDisplay(resources)
  expect(actual).toEqual(expectedTuple)
})

it("returns null if there are no pending builds and the most recent build has no edits", () => {
  let recent = {
    startTime: "2019-04-24T13:08:41.017623-04:00",
    finishTime: "2019-04-24T13:08:42.926608-04:00",
    isCrashRebuild: false,
    warnings: [],
  }
  let expectedTuple = {
    name: "snack",
    edits: ["main.go"],
    since: recent.startTime,
  }
  const resource: ResourceWithBuilds = {
    name: "snack",
    buildHistory: [
      {
        startTime: "2019-04-24T13:08:39.017623-04:00",
        finishTime: "2019-04-24T13:08:40.926608-04:00",
        isCrashRebuild: false,
        warnings: [],
      },
      recent,
    ],
    pendingBuildEdits: null,
    pendingBuildSince: zeroTime,
  }
  const resources: ResourceWithBuilds[] = [resource]

  let actual = mostRecentBuildToDisplay(resources)
  expect(actual).toBeNull()
})

it("returns the pending build if there is one", () => {
  let expectedTuple = {
    name: "snack",
    edits: ["bar"],
    since: "2019-04-24T13:08:41.017623-04:00",
  }
  const resource: ResourceWithBuilds = {
    name: "snack",
    buildHistory: [
      {
        startTime: "2019-04-24T13:08:39.017623-04:00",
        finishTime: "2019-04-24T13:08:40.926608-04:00",
        isCrashRebuild: false,
        warnings: [],
      },
    ],
    pendingBuildEdits: ["bar"],
    pendingBuildSince: "2019-04-24T13:08:41.017623-04:00",
  }
  const resources = [resource]

  let actual = mostRecentBuildToDisplay(resources)
  expect(actual).toEqual(expectedTuple)
})
