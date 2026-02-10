import {
  fireEvent,
  render,
  RenderOptions,
  screen,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import fetchMock from "fetch-mock"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter } from "react-router"
import BuildButton, { StartBuildButtonProps } from "./BuildButton"
import { oneUIButton } from "./testdata"
import { BuildButtonTooltip, startBuild } from "./trigger"
import { TriggerMode } from "./types"

function expectClickable(button: HTMLElement, expected: boolean) {
  if (expected) {
    expect(button).toHaveClass("is-clickable")
    expect(button).not.toBeDisabled()
  } else {
    expect(button).not.toHaveClass("is-clickable")
    expect(button).toBeDisabled()
  }
}
function expectManualStartBuildIcon(expected: boolean) {
  const iconId = expected ? "build-manual-icon" : "build-auto-icon"
  expect(screen.getByTestId(iconId)).toBeInTheDocument()
}
function expectIsSelected(button: HTMLElement, expected: boolean) {
  if (expected) {
    expect(button).toHaveClass("is-selected")
  } else {
    expect(button).not.toHaveClass("is-selected")
  }
}
function expectIsQueued(button: HTMLElement, expected: boolean) {
  if (expected) {
    expect(button).toHaveClass("is-queued")
  } else {
    expect(button).not.toHaveClass("is-queued")
  }
}
function expectWithTooltip(expected: string) {
  expect(screen.getByTitle(expected)).toBeInTheDocument()
}

const stopBuildButton = oneUIButton({ buttonName: "stopBuild" })

function customRender(
  buttonProps: Partial<StartBuildButtonProps>,
  options?: RenderOptions
) {
  return render(
    <BuildButton
      stopBuildButton={stopBuildButton}
      onStartBuild={buttonProps.onStartBuild ?? (() => {})}
      hasBuilt={buttonProps.hasBuilt ?? false}
      isBuilding={buttonProps.isBuilding ?? false}
      isSelected={buttonProps.isSelected}
      isQueued={buttonProps.isQueued ?? false}
      hasPendingChanges={buttonProps.hasPendingChanges ?? false}
      triggerMode={buttonProps.triggerMode ?? TriggerMode.TriggerModeAuto}
    />,
    {
      wrapper: ({ children }) => (
        <MemoryRouter
          initialEntries={["/"]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          <SnackbarProvider>{children}</SnackbarProvider>
        </MemoryRouter>
      ),
      ...options,
    }
  )
}

describe("SidebarBuildButton", () => {
  beforeEach(() => {
    fetchMock.mock("/api/trigger", JSON.stringify({}))
  })

  afterEach(() => {
    fetchMock.reset()
  })

  describe("start builds", () => {
    it("POSTs to endpoint when clicked", () => {
      customRender({
        onStartBuild: () => startBuild("doggos"),
        hasBuilt: true,
      })

      const buildButton = screen.getByLabelText(BuildButtonTooltip.Default)
      expect(buildButton).toBeInTheDocument()

      // Construct a mouse event with method spies
      const preventDefault = jest.fn()
      const stopPropagation = jest.fn()
      const clickEvent = new MouseEvent("click", { bubbles: true })
      clickEvent.preventDefault = preventDefault
      clickEvent.stopPropagation = stopPropagation

      fireEvent(buildButton, clickEvent)

      expect(preventDefault).toHaveBeenCalled()
      expect(stopPropagation).toHaveBeenCalled()

      expect(fetchMock.calls().length).toEqual(1)
      expect(fetchMock.calls()[0][0]).toEqual("/api/trigger")
      expect(fetchMock.calls()[0][1]?.method).toEqual("post")
      expect(fetchMock.calls()[0][1]?.body).toEqual(
        JSON.stringify({
          manifest_names: ["doggos"],
          build_reason: 16 /* BuildReasonFlagTriggerWeb */,
        })
      )
    })

    it("disables button when resource is queued", () => {
      const startBuildSpy = jest.fn()
      customRender({ isQueued: true, onStartBuild: startBuildSpy })

      const buildButton = screen.getByLabelText(
        BuildButtonTooltip.AlreadyQueued
      )
      expect(buildButton).toBeDisabled()

      userEvent.click(buildButton, undefined, { skipPointerEventsCheck: true })

      expect(startBuildSpy).not.toHaveBeenCalled()
    })

    it("shows the button for TriggerModeManual", () => {
      const startBuildSpy = jest.fn()
      customRender({
        triggerMode: TriggerMode.TriggerModeManual,
        onStartBuild: startBuildSpy,
      })

      expectManualStartBuildIcon(true)
    })

    test.each([true, false])(
      "shows clickable + bold start build button for manual resource. hasPendingChanges: %s",
      (hasPendingChanges) => {
        customRender({
          triggerMode: TriggerMode.TriggerModeManual,
          hasPendingChanges,
          hasBuilt: !hasPendingChanges,
        })

        const tooltipText = hasPendingChanges
          ? BuildButtonTooltip.NeedsManualTrigger
          : BuildButtonTooltip.Default
        const buildButton = screen.getByLabelText(tooltipText)

        expect(buildButton).toBeInTheDocument()
        expectClickable(buildButton, true)
        expectManualStartBuildIcon(hasPendingChanges)
        expectIsQueued(buildButton, false)
        expectWithTooltip(tooltipText)
      }
    )

    test.each([true, false])(
      "shows selected trigger button for resource is selected: %p",
      (isSelected) => {
        customRender({ isSelected, hasBuilt: true })

        const buildButton = screen.getByLabelText(BuildButtonTooltip.Default)

        expect(buildButton).toBeInTheDocument()
        expectIsSelected(buildButton, isSelected) // Selected resource
      }
    )

    // A pending resource may mean that a pod is being rolled out, but is not
    // ready yet. In that case, the start build button will delete the pod (cancelling
    // the rollout) and rebuild.
    it("shows start build button when pending but no current build", () => {
      customRender({ hasPendingChanges: true, hasBuilt: true })

      const buildButton = screen.getByLabelText(BuildButtonTooltip.Default)

      expect(buildButton).toBeInTheDocument()
      expectClickable(buildButton, true)
      expectManualStartBuildIcon(false)
      expectIsQueued(buildButton, false)
      expectWithTooltip(BuildButtonTooltip.Default)
    })

    it("renders an unclickable start build button if resource waiting for first build", () => {
      customRender({})

      const buildButton = screen.getByLabelText(
        BuildButtonTooltip.UpdateInProgOrPending
      )

      expect(buildButton).toBeInTheDocument()
      expectClickable(buildButton, false)
      expectManualStartBuildIcon(false)
      expectIsQueued(buildButton, false)
      expectWithTooltip(BuildButtonTooltip.UpdateInProgOrPending)
    })

    it("renders queued resource with class .isQueued and NOT .clickable", () => {
      customRender({ isQueued: true })

      const buildButton = screen.getByLabelText(
        BuildButtonTooltip.AlreadyQueued
      )

      expect(buildButton).toBeInTheDocument()
      expectClickable(buildButton, false)
      expectManualStartBuildIcon(false)
      expectIsQueued(buildButton, true)
      expectWithTooltip(BuildButtonTooltip.AlreadyQueued)
    })
  })

  describe("stop builds", () => {
    it("renders a stop button when the build is in progress", () => {
      customRender({ isBuilding: true })

      const buildButton = screen.getByLabelText(
        `Trigger ${stopBuildButton.spec?.text}`
      )

      expect(buildButton).toBeInTheDocument()
      // The button group has the .stop-button class
      expect(screen.getByRole("group")).toHaveClass("stop-button")
      expectWithTooltip(BuildButtonTooltip.Stop)
    })
  })
})
