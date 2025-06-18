import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import fetchMock from "fetch-mock"
import OverviewTableTriggerModeToggle, {
  ToggleTriggerModeTooltip,
} from "./OverviewTableTriggerModeToggle"
import { TriggerMode } from "./types"

function mockTriggerModeCalls() {
  fetchMock.mock(
    (url) => url.startsWith("/api/override/trigger_mode"),
    JSON.stringify({})
  )
}

describe("OverviewTableTriggerModeToggle", () => {
  beforeEach(() => {
    mockTriggerModeCalls()
  })

  afterEach(() => {
    fetchMock.reset()
  })

  test.each([TriggerMode.TriggerModeManual, TriggerMode.TriggerModeAuto])(
    "sets trigger mode on click when trigger mode is %s",
    (triggerMode) => {
      render(
        <OverviewTableTriggerModeToggle
          resourceName="foo"
          triggerMode={triggerMode}
        />
      )

      const tooltipText =
        triggerMode == TriggerMode.TriggerModeAuto
          ? ToggleTriggerModeTooltip.isAuto
          : ToggleTriggerModeTooltip.isManual
      const triggerModeButton = screen.getByTitle(tooltipText)
      userEvent.click(triggerModeButton)

      const calls = fetchMock.calls()
      expect(calls.length).toEqual(1)
      const call = calls[0]
      expect(call[0]).toEqual("/api/override/trigger_mode")
      expect(call[1]).toBeTruthy()
      expect(call[1]!.method).toEqual("post")
      expect(call[1]!.body).toBeTruthy()
      const request = JSON.parse(call[1]!.body!.toString())
      let expectedTriggerMode: TriggerMode
      switch (triggerMode) {
        case TriggerMode.TriggerModeAuto:
          expectedTriggerMode = TriggerMode.TriggerModeManual
          break
        case TriggerMode.TriggerModeManual:
          expectedTriggerMode = TriggerMode.TriggerModeAuto
          break
        default:
          fail(`unknown trigger mode: ${triggerMode}`)
      }
      expect(request).toEqual({
        manifest_names: ["foo"],
        trigger_mode: expectedTriggerMode,
      })
    }
  )
})
