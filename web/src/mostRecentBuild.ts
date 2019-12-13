import { isZeroTime } from "./time"

type Build = Proto.webviewBuildRecord

export type ResourceWithBuilds = {
  name: string
  buildHistory: Build[]
  pendingBuildSince: string
  pendingBuildEdits: string[] | null
}

const buildByDate = (b1: BuildTuple, b2: BuildTuple) => {
  let b1Date = Date.parse(b1.since)
  let b2Date = Date.parse(b2.since)
  if (b1Date > b2Date) {
    return -1
  }
  if (b1Date < b2Date) {
    return 1
  }
  return 0
}

type BuildTuple = {
  name: string
  since: string
  edits: string[]
}

const makePendingBuild = (r: ResourceWithBuilds): BuildTuple => {
  return {
    name: r.name,
    since: r.pendingBuildSince ?? "",
    edits: r.pendingBuildEdits || [],
  }
}

const makeBuildHistory = (r: ResourceWithBuilds, b: Build): BuildTuple => {
  return {
    name: r.name,
    since: b.startTime ?? "",
    edits: b.edits || [],
  }
}

const mostRecentBuildToDisplay = (
  resources: ResourceWithBuilds[]
): BuildTuple | null => {
  let r = null
  let pendingBuildsSorted = resources
    .map(r => makePendingBuild(r))
    .filter(b => !isZeroTime(b.since))
    .sort(buildByDate)

  if (pendingBuildsSorted.length > 0) {
    return pendingBuildsSorted[0]
  }

  let buildHistorySorted = resources
    .flatMap(r => r.buildHistory.map(b => makeBuildHistory(r, b)))
    .sort(buildByDate)

  if (buildHistorySorted.length > 0 && buildHistorySorted[0].edits.length > 0) {
    return buildHistorySorted[0]
  }

  return r
}

export default mostRecentBuildToDisplay
