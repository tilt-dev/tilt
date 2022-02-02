import {
  render,
  screen,
  waitForElementToBeRemoved,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import {
  cleanupMockAnalyticsCalls,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { buttonsByComponent } from "./ApiButton"
import { mockUIButtonUpdates } from "./ApiButton.testhelpers"
import Features, { FeaturesProvider, Flag } from "./feature"
import {
  BulkAction,
  buttonsByAction,
  OverviewTableBulkActions,
} from "./OverviewTableBulkActions"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { disableButton, oneButton } from "./testdata"

const TEST_SELECTIONS = ["frontend", "backend"]

const TEST_UIBUTTONS = [
  oneButton(0, "database"),
  disableButton("frontend", true),
  oneButton(2, "frontend"),
  disableButton("backend", false),
]

const OverviewTableBulkActionsTestWrapper = (props: {
  flagEnabled: boolean
  resourceSelections: string[]
}) => {
  const { flagEnabled, resourceSelections } = props
  const features = new Features({ [Flag.DisableResources]: flagEnabled })
  return (
    <FeaturesProvider value={features}>
      <ResourceSelectionProvider initialValuesForTesting={resourceSelections}>
        <OverviewTableBulkActions uiButtons={TEST_UIBUTTONS} />
      </ResourceSelectionProvider>
    </FeaturesProvider>
  )
}

describe("OverviewTableBulkActions", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    mockUIButtonUpdates()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  describe("when disable resources are NOT enabled", () => {
    it("does NOT display", () => {
      render(
        <OverviewTableBulkActionsTestWrapper
          flagEnabled={false}
          resourceSelections={TEST_SELECTIONS}
        />
      )

      expect(screen.queryByLabelText("Bulk resource actions")).toBeNull()
    })
  })

  describe("when disable resources are enabled", () => {
    describe("when there are NO resources selected", () => {
      it("does NOT display", () => {
        render(
          <OverviewTableBulkActionsTestWrapper
            flagEnabled={true}
            resourceSelections={[]}
          />
        )

        expect(screen.queryByLabelText("Bulk resource actions")).toBeNull()
      })
    })

    describe("when there are resources selected", () => {
      beforeEach(() => {
        render(
          <OverviewTableBulkActionsTestWrapper
            flagEnabled={true}
            resourceSelections={TEST_SELECTIONS}
          />
        )
      })

      it("does display", () => {
        expect(screen.queryByLabelText("Bulk resource actions")).not.toBeNull()
      })

      it("displays the selected resource count", () => {
        expect(
          screen.queryByText(`${TEST_SELECTIONS.length} selected`)
        ).not.toBeNull()
      })

      it("renders an 'Enable' button that does NOT require confirmation", async () => {
        const enableButton = screen.queryByLabelText("Trigger Enable")
        expect(enableButton).toBeTruthy()

        // Clicking the button should NOT bring up a confirmation step
        userEvent.click(enableButton as HTMLElement)

        // Clicking an action button will remove the selected resources
        // and the bulk action bar will no longer appear
        await waitForElementToBeRemoved(
          screen.queryByLabelText("Trigger Enable")
        )
      })

      it("renders a 'Disable' button that does require confirmation", () => {
        const disableButton = screen.queryByLabelText("Trigger Disable")
        expect(disableButton).toBeTruthy()

        // Clicking the button should bring up a confirmation step
        userEvent.click(disableButton as HTMLElement)

        expect(screen.queryByLabelText("Confirm Disable")).toBeTruthy()
        expect(screen.queryByLabelText("Cancel Disable")).toBeTruthy()
      })

      it("clears the selected resources after a button has been clicked", async () => {
        const enableButton = screen.queryByLabelText("Trigger Enable")
        expect(enableButton).toBeTruthy()

        // Click the enable button, since there's not the extra confirmation step
        userEvent.click(enableButton as HTMLElement)

        // Expect that the component doesn't appear any more since there are no selections
        await waitForElementToBeRemoved(
          screen.queryByLabelText("Bulk resource actions")
        )
      })
    })
  })

  describe("buttonsByAction", () => {
    it("groups UIButtons by the bulk action they perform", () => {
      const componentButtons = buttonsByComponent(TEST_UIBUTTONS)
      const actionButtons = buttonsByAction(componentButtons, TEST_SELECTIONS)

      expect(actionButtons).toStrictEqual({
        [BulkAction.Disable]: [TEST_UIBUTTONS[1], TEST_UIBUTTONS[3]],
      })
    })
  })
})
