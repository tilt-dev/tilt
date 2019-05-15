import {
  oneResourceBuilding,
  oneResourceFailedToBuild,
  oneResourceCrashedOnStart,
  oneResourceNoAlerts,
} from "./testdata.test"
import { combinedStatusMessage } from "./combinedStatusMessage"
import { StatusItem } from "./Statusbar"

describe("combined status message", () => {
  it("should show that there's one resource building", () => {
    let data = oneResourceBuilding()
    let resources = data.map(r => new StatusItem(r))
    let actual = combinedStatusMessage(resources)

    expect(actual).toBe("Updating snack…")
  })

  it("should show the most recent resource that failed to build", () => {
    let data = oneResourceFailedToBuild()
    let resources = data.map((r: any) => new StatusItem(r))
    let actual = combinedStatusMessage(resources)

    expect(actual).toBe("Build failed: snack")
  })

  it("should show the most recent resource that crashed on start", () => {
    let data = oneResourceCrashedOnStart()
    let resources = data.map((r: any) => new StatusItem(r))
    let actual = combinedStatusMessage(resources)

    expect(actual).toBe("Container crashed: snack")
  })

  it("should show nothing if all is good", () => {
    let resource = oneResourceNoAlerts()
    let data = [resource]
    let resources = data.map((r: any) => new StatusItem(r))
    let actual = combinedStatusMessage(resources)

    expect(actual).toBe("")
  })
})
