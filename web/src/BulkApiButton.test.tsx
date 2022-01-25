import { ApiButtonToggleState } from "./ApiButton"
import { canBulkButtonBeToggled, canButtonBeToggled } from "./BulkApiButton"
import { disableButton, oneButton } from "./testdata"

const NON_TOGGLE_BUTTON = oneButton(0, "database")
const DISABLE_BUTTON = disableButton("frontend", true)
const ENABLE_BUTTON = disableButton("backend", false)

describe("BulkApiButton", () => {
  describe("canButtonBeToggled", () => {
    it("returns true when there is no target toggle state", () => {
      expect(canButtonBeToggled(DISABLE_BUTTON)).toBe(true)
    })

    it("returns false when button is not a toggle button", () => {
      expect(canButtonBeToggled(NON_TOGGLE_BUTTON)).toBe(false)
    })

    it("returns false when button is already in the target toggle state", () => {
      expect(canButtonBeToggled(DISABLE_BUTTON, ApiButtonToggleState.On)).toBe(
        false
      )
      expect(canButtonBeToggled(ENABLE_BUTTON, ApiButtonToggleState.Off)).toBe(
        false
      )
    })

    it("returns true when button is not in the target toggle state", () => {
      expect(canButtonBeToggled(DISABLE_BUTTON, ApiButtonToggleState.Off)).toBe(
        true
      )
      expect(canButtonBeToggled(ENABLE_BUTTON, ApiButtonToggleState.On)).toBe(
        true
      )
    })
  })

  describe("canBulkButtonBeToggled", () => {
    it("returns false if no buttons in the list of buttons can be toggled", () => {
      expect(
        canBulkButtonBeToggled(
          [NON_TOGGLE_BUTTON, DISABLE_BUTTON],
          ApiButtonToggleState.On
        )
      ).toBe(false)
    })

    it("returns true if at least one button in the list of buttons can be toggled", () => {
      expect(
        canBulkButtonBeToggled(
          [NON_TOGGLE_BUTTON, DISABLE_BUTTON, ENABLE_BUTTON],
          ApiButtonToggleState.On
        )
      ).toBe(true)
    })
  })
})
