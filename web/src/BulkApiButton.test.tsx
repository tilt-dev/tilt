import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AnalyticsAction, AnalyticsType } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
  nonAnalyticsCalls,
} from "./analytics_test_helpers"
import { ApiButtonToggleState, ApiButtonType } from "./ApiButton"
import {
  getUIButtonDataFromCall,
  mockUIButtonUpdates,
} from "./ApiButton.testhelpers"
import {
  BulkApiButton,
  canBulkButtonBeToggled,
  canButtonBeToggled,
} from "./BulkApiButton"
import { BulkAction } from "./OverviewTableBulkActions"
import { flushPromises } from "./promise"
import { disableButton, oneButton } from "./testdata"

const NON_TOGGLE_BUTTON = oneButton(0, "database")
const DISABLE_BUTTON_DB = disableButton("database", true)
const DISABLE_BUTTON_FRONTEND = disableButton("frontend", true)
const ENABLE_BUTTON_BACKEND = disableButton("backend", false)
const TEST_UIBUTTONS = [
  DISABLE_BUTTON_DB,
  DISABLE_BUTTON_FRONTEND,
  ENABLE_BUTTON_BACKEND,
]

describe("BulkApiButton", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    mockUIButtonUpdates()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  it("is disabled when there are no UIButtons", () => {
    render(
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="I cannot be clicked"
        requiresConfirmation={false}
        uiButtons={[]}
      />
    )

    const bulkButton = screen.getByLabelText("Trigger I cannot be clicked")

    expect(bulkButton).toBeTruthy()
    expect((bulkButton as HTMLButtonElement).disabled).toBe(true)
  })

  it("is disabled when there are no UIButtons that can be toggled to the target toggle state", () => {
    render(
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="I cannot be toggled that way"
        requiresConfirmation={false}
        targetToggleState={ApiButtonToggleState.On}
        uiButtons={[DISABLE_BUTTON_DB]}
      />
    )

    const bulkButton = screen.getByLabelText(
      "Trigger I cannot be toggled that way"
    )

    expect(bulkButton).toBeTruthy()
    expect((bulkButton as HTMLButtonElement).disabled).toBe(true)
  })

  it("is enabled when there are UIButtons", () => {
    render(
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Run lint"
        requiresConfirmation={false}
        uiButtons={[NON_TOGGLE_BUTTON]}
      />
    )

    const bulkButton = screen.getByLabelText("Trigger Run lint")

    expect(bulkButton).toBeTruthy()
    expect((bulkButton as HTMLButtonElement).disabled).toBe(false)
  })

  it("is enabled when there are UIButtons that can be toggled to the target toggle state", () => {
    render(
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Enable resources"
        requiresConfirmation={false}
        targetToggleState={ApiButtonToggleState.On}
        uiButtons={[DISABLE_BUTTON_DB, ENABLE_BUTTON_BACKEND]}
      />
    )

    const bulkButton = screen.getByLabelText("Trigger Enable resources")

    expect(bulkButton).toBeTruthy()
    expect((bulkButton as HTMLButtonElement).disabled).toBe(false)
  })

  describe("when it's clicked", () => {
    let mockCallback: jest.Mock

    beforeEach(async () => {
      mockCallback = jest.fn()
      render(
        <BulkApiButton
          bulkAction={BulkAction.Disable}
          buttonText="Turn everything off"
          onClickCallback={mockCallback}
          requiresConfirmation={false}
          targetToggleState={ApiButtonToggleState.Off}
          uiButtons={TEST_UIBUTTONS}
        />
      )

      const bulkButton = screen.getByLabelText("Trigger Turn everything off")
      userEvent.click(bulkButton)

      // Wait for the async calls to complete
      await waitFor(flushPromises)
    })

    it("triggers all buttons that can be toggled when it's clicked", () => {
      const buttonUpdateCalls = nonAnalyticsCalls()

      // Out of the three test buttons, only two of them can be toggled to the target toggle state
      expect(buttonUpdateCalls.length).toBe(2)

      const buttonUpdateNames = buttonUpdateCalls.map(
        (call) => getUIButtonDataFromCall(call)?.metadata?.name
      )

      expect(buttonUpdateNames).toStrictEqual([
        DISABLE_BUTTON_DB.metadata?.name,
        DISABLE_BUTTON_FRONTEND.metadata?.name,
      ])
    })

    it("generates the correct analytics payload", () => {
      expectIncrs({
        name: "ui.web.bulkButton",
        tags: {
          action: AnalyticsAction.Click,
          bulkAction: BulkAction.Disable,
          bulkCount: "3",
          toggleValue: ApiButtonToggleState.On,
          component: ApiButtonType.Global,
          type: AnalyticsType.Grid,
        },
      })
    })

    it("calls a specified onClick callback", () => {
      expect(mockCallback).toHaveBeenCalledTimes(1)
    })
  })

  describe("when it requires confirmation", () => {
    beforeEach(async () => {
      render(
        <BulkApiButton
          bulkAction={BulkAction.Disable}
          buttonText="Click everything when I'm sure"
          requiresConfirmation={true}
          uiButtons={TEST_UIBUTTONS}
        />
      )

      const bulkButton = screen.getByLabelText(
        "Trigger Click everything when I'm sure"
      )
      userEvent.click(bulkButton)
    })

    it("displays confirm and cancel buttons when clicked once", () => {
      expect(
        screen.getByLabelText("Confirm Click everything when I'm sure")
      ).toBeTruthy()
      expect(
        screen.getByLabelText("Cancel Click everything when I'm sure")
      ).toBeTruthy()
    })

    it("triggers all buttons when `Confirm` is clicked", async () => {
      userEvent.click(
        screen.getByLabelText("Confirm Click everything when I'm sure")
      )

      await waitFor(flushPromises)

      const buttonUpdateCalls = nonAnalyticsCalls()
      expect(buttonUpdateCalls.length).toBe(3)

      const buttonUpdateNames = buttonUpdateCalls.map(
        (call) => getUIButtonDataFromCall(call)?.metadata?.name
      )

      expect(buttonUpdateNames).toStrictEqual([
        DISABLE_BUTTON_DB.metadata?.name,
        DISABLE_BUTTON_FRONTEND.metadata?.name,
        ENABLE_BUTTON_BACKEND.metadata?.name,
      ])
    })

    it("does NOT trigger any buttons when `Cancel` is clicked", async () => {
      userEvent.click(
        screen.getByLabelText("Cancel Click everything when I'm sure")
      )

      // There shouldn't be any async calls made when canceling, but wait in case
      await waitFor(flushPromises)

      expect(
        screen.getByLabelText("Trigger Click everything when I'm sure")
      ).toBeTruthy()
    })

    it("generates the correct analytics payload for confirmation", async () => {
      userEvent.click(
        screen.getByLabelText("Confirm Click everything when I'm sure")
      )
      await waitFor(flushPromises)

      expectIncrs(
        {
          name: "ui.web.bulkButton",
          tags: {
            action: AnalyticsAction.Click,
            bulkAction: BulkAction.Disable,
            bulkCount: "3",
            component: ApiButtonType.Global,
            type: AnalyticsType.Grid,
          },
        },
        {
          name: "ui.web.bulkButton",
          tags: {
            action: AnalyticsAction.Click,
            bulkAction: BulkAction.Disable,
            bulkCount: "3",
            confirm: "true",
            component: ApiButtonType.Global,
            type: AnalyticsType.Grid,
          },
        }
      )
    })

    it("generates the correct analytics payload for cancelation", () => {
      userEvent.click(
        screen.getByLabelText("Cancel Click everything when I'm sure")
      )

      expectIncrs(
        {
          name: "ui.web.bulkButton",
          tags: {
            action: AnalyticsAction.Click,
            bulkAction: BulkAction.Disable,
            bulkCount: "3",
            component: ApiButtonType.Global,
            type: AnalyticsType.Grid,
          },
        },
        {
          name: "ui.web.bulkButton",
          tags: {
            action: AnalyticsAction.Click,
            bulkAction: BulkAction.Disable,
            bulkCount: "3",
            confirm: "false",
            component: ApiButtonType.Global,
            type: AnalyticsType.Grid,
          },
        }
      )
    })
  })

  describe("helpers", () => {
    describe("canButtonBeToggled", () => {
      it("returns true when there is no target toggle state", () => {
        expect(canButtonBeToggled(DISABLE_BUTTON_FRONTEND)).toBe(true)
      })

      it("returns false when button is not a toggle button", () => {
        expect(canButtonBeToggled(NON_TOGGLE_BUTTON)).toBe(false)
      })

      it("returns false when button is already in the target toggle state", () => {
        expect(
          canButtonBeToggled(DISABLE_BUTTON_FRONTEND, ApiButtonToggleState.On)
        ).toBe(false)
        expect(
          canButtonBeToggled(ENABLE_BUTTON_BACKEND, ApiButtonToggleState.Off)
        ).toBe(false)
      })

      it("returns true when button is not in the target toggle state", () => {
        expect(
          canButtonBeToggled(DISABLE_BUTTON_FRONTEND, ApiButtonToggleState.Off)
        ).toBe(true)
        expect(
          canButtonBeToggled(ENABLE_BUTTON_BACKEND, ApiButtonToggleState.On)
        ).toBe(true)
      })
    })

    describe("canBulkButtonBeToggled", () => {
      it("returns false if no buttons in the list of buttons can be toggled", () => {
        expect(
          canBulkButtonBeToggled(
            [NON_TOGGLE_BUTTON, DISABLE_BUTTON_FRONTEND],
            ApiButtonToggleState.On
          )
        ).toBe(false)
      })

      it("returns true if at least one button in the list of buttons can be toggled", () => {
        expect(
          canBulkButtonBeToggled(
            [NON_TOGGLE_BUTTON, DISABLE_BUTTON_FRONTEND, ENABLE_BUTTON_BACKEND],
            ApiButtonToggleState.On
          )
        ).toBe(true)
      })
    })
  })
})
