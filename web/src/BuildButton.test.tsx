import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import BuildButton, { BuildButtonProps } from "./BuildButton"
import { InstrumentedButton } from "./instrumentedComponents"
import TiltTooltip from "./Tooltip"
import { BuildButtonTooltip, startBuild } from "./trigger"
import { TriggerMode } from "./types"

function expectClickable(button: any, expected: boolean) {
  const ib = button.find(InstrumentedButton)
  expect(ib.hasClass("is-clickable")).toEqual(expected)
  expect(ib.prop("disabled")).toEqual(!expected)
}
function expectManualStartBuildIcon(button: any, expected: boolean) {
  let icon = expected
    ? "start-build-button-manual.svg"
    : "start-build-button.svg"
  expect(button.find(InstrumentedButton).getDOMNode().innerHTML).toContain(icon)
}
function expectIsSelected(button: any, expected: boolean) {
  expect(button.find(InstrumentedButton).hasClass("is-selected")).toEqual(
    expected
  )
}
function expectIsQueued(button: any, expected: boolean) {
  expect(button.find(InstrumentedButton).hasClass("is-queued")).toEqual(
    expected
  )
}
function expectWithTooltip(button: any, expected: string) {
  expect(button.find(TiltTooltip).prop("title")).toEqual(expected)
}

function BuildButtonTestWrapper(props: Partial<BuildButtonProps>) {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <SnackbarProvider>
        <BuildButton
          onStartBuild={props.onStartBuild ?? (() => {})}
          hasBuilt={props.hasBuilt ?? false}
          isBuilding={props.isBuilding ?? false}
          isSelected={props.isSelected}
          isQueued={props.isQueued ?? false}
          hasPendingChanges={props.hasPendingChanges ?? false}
          triggerMode={props.triggerMode ?? TriggerMode.TriggerModeAuto}
          analyticsTags={props.analyticsTags ?? {}}
        />
      </SnackbarProvider>
    </MemoryRouter>
  )
}

describe("SidebarBuildButton", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    fetchMock.mock("/api/trigger", JSON.stringify({}))
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  describe("start builds", () => {
    it("POSTs to endpoint when clicked", () => {
      const root = mount(
        <BuildButtonTestWrapper
          onStartBuild={() => startBuild("doggos")}
          hasBuilt={true}
          analyticsTags={{ target: "k8s" }}
        />
      )

      let element = root.find(InstrumentedButton)
      expect(element).toHaveLength(1)

      let preventDefaulted = false
      element.simulate("click", {
        preventDefault: () => {
          preventDefaulted = true
        },
      })
      expect(preventDefaulted).toEqual(true)

      expectIncrs({
        name: "ui.web.triggerResource",
        tags: { action: AnalyticsAction.Click, target: "k8s" },
      })

      expect(fetchMock.calls().length).toEqual(2)
      expect(fetchMock.calls()[1][0]).toEqual("/api/trigger")
      expect(fetchMock.calls()[1][1]?.method).toEqual("post")
      expect(fetchMock.calls()[1][1]?.body).toEqual(
        JSON.stringify({
          manifest_names: ["doggos"],
          build_reason: 16 /* BuildReasonFlagTriggerWeb */,
        })
      )
    })

    it("disables button when resource is queued", () => {
      const root = mount(
        <BuildButtonTestWrapper
          isQueued={true}
          onStartBuild={() => startBuild("doggos")}
        />
      )

      let element = root.find(BuildButton).find(InstrumentedButton)
      expect(element).toHaveLength(1)
      element.simulate("click")

      expect(fetchMock.calls().length).toEqual(0)
    })

    it("shows the button for TriggerModeManual", () => {
      const root = mount(
        <BuildButtonTestWrapper
          triggerMode={TriggerMode.TriggerModeManual}
          onStartBuild={() => startBuild("doggos")}
        />
      )

      let element = root.find(BuildButton).find(InstrumentedButton)
      expectManualStartBuildIcon(element, true)

      expect(element).toHaveLength(1)
      element.simulate("click")

      expect(fetchMock.calls().length).toEqual(2)
    })

    test.each([true, false])(
      "shows clickable + bold start build button for manual resource. hasPendingChanges: %b",
      (hasPendingChanges) => {
        const root = mount(
          <BuildButtonTestWrapper
            triggerMode={TriggerMode.TriggerModeManual}
            hasPendingChanges={hasPendingChanges}
            hasBuilt={!hasPendingChanges}
          />
        )

        let buttons = root.find(BuildButton)
        expect(buttons).toHaveLength(1)

        let b = buttons.at(0) // Manual resource with pending changes

        expectClickable(b, true)
        expectManualStartBuildIcon(b, hasPendingChanges)
        expectIsQueued(b, false)
        if (hasPendingChanges) {
          expectWithTooltip(b, BuildButtonTooltip.NeedsManualTrigger)
        } else {
          expectWithTooltip(b, BuildButtonTooltip.Default)
        }
      }
    )

    test.each([true, false])(
      "shows selected trigger button for resource is selected: %p",
      (isSelected) => {
        const root = mount(<BuildButtonTestWrapper isSelected={isSelected} />)

        let buttons = root.find(BuildButton)
        expect(buttons).toHaveLength(1)

        expectIsSelected(buttons.at(0), isSelected) // Selected resource
      }
    )

    // A pending resource may mean that a pod is being rolled out, but is not
    // ready yet. In that case, the start build button will delete the pod (cancelling
    // the rollout) and rebuild.
    it("shows start build button when pending but no current build", () => {
      const root = mount(
        <BuildButtonTestWrapper hasPendingChanges={true} hasBuilt={true} />
      )

      let buttons = root.find(BuildButton)
      expect(buttons).toHaveLength(1)
      let b = buttons.at(0) // Automatic resource with pending changes

      expectClickable(b, true)
      expectManualStartBuildIcon(b, false)
      expectIsQueued(b, false)
      expectWithTooltip(b, BuildButtonTooltip.Default)
    })

    it("renders an unclickable start build button if resource waiting for first build", () => {
      const root = mount(<BuildButtonTestWrapper />)

      let button = root.find(BuildButton)
      expect(button).toHaveLength(1)

      expectClickable(button, false)
      expectManualStartBuildIcon(button, false)
      expectIsQueued(button, false)
      expectWithTooltip(button, BuildButtonTooltip.UpdateInProgOrPending)
    })

    it("renders queued resource with class .isQueued and NOT .clickable", () => {
      const root = mount(<BuildButtonTestWrapper isQueued={true} />)

      let button = root.find(BuildButton)
      expect(button).toHaveLength(1)

      expectClickable(button, false)
      expectManualStartBuildIcon(button, false)
      expectIsQueued(button, true)
      expectWithTooltip(button, BuildButtonTooltip.AlreadyQueued)
    })
  })
})
