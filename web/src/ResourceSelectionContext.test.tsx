import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React, { ChangeEvent, useState } from "react"
import {
  ResourceSelectionProvider,
  useResourceSelection,
} from "./ResourceSelectionContext"

const SELECTION_STATE = "selected-state"
const IS_SELECTED = "selected-text"
const TO_SELECT_INPUT = "select-input"
const SELECT_BUTTON = "select-button"
const DESELECT_BUTTON = "deselect-button"
const CLEAR_BUTTON = "clear-button"

// This is a very basic test component that prints out the state
// from the ResourceSelection context and provides buttons to trigger
// methods returned by the context, so they can be tested
const TestConsumer = () => {
  const { selected, isSelected, select, deselect, clearSelections } =
    useResourceSelection()

  const [value, setValue] = useState("")
  const onChange = (e: ChangeEvent<HTMLInputElement>) =>
    setValue(e.target.value)

  // To support selecting multiple items at once, accept a comma separated list
  const parsedValues = (value: string) => value.split(",")
  return (
    <>
      {/* Print the `selected` state */}
      <p aria-label={SELECTION_STATE}>{JSON.stringify(Array.from(selected))}</p>
      {/* Use an input field to change the resource that can be selected/deselected */}
      <input aria-label={TO_SELECT_INPUT} type="text" onChange={onChange} />
      {/* Print the state for whatever resource is currently in the `input` */}
      <p aria-label={IS_SELECTED}>{isSelected(value).toString()}</p>
      {/* Select the resource that's currently in the `input` */}
      <button
        aria-label={SELECT_BUTTON}
        onClick={() => select(...parsedValues(value))}
      >
        Select
      </button>
      {/* Deselect the resource that's currently in the `input` */}
      <button
        aria-label={DESELECT_BUTTON}
        onClick={() => deselect(...parsedValues(value))}
      >
        Deselect
      </button>
      {/* Clear all selections */}
      <button aria-label={CLEAR_BUTTON} onClick={() => clearSelections()}>
        Clear selections
      </button>
    </>
  )
}

describe("ResourceSelectionContext", () => {
  // Helpers
  const INITIAL_SELECTIONS = ["vigoda", "magic_beans", "servantes"]
  const selectedState = () =>
    screen.queryByLabelText(SELECTION_STATE)?.textContent
  const isSelected = () => screen.queryByLabelText(IS_SELECTED)?.textContent
  const setCurrentResource = (value: string) => {
    const inputField = screen.queryByLabelText(TO_SELECT_INPUT)
    userEvent.type(inputField as HTMLInputElement, value)
  }
  const selectResource = (value: string) => {
    setCurrentResource(value)
    const selectButton = screen.queryByLabelText(SELECT_BUTTON)
    userEvent.click(selectButton as HTMLButtonElement)
  }
  const deselectResource = (value: string) => {
    setCurrentResource(value)
    const deselectButton = screen.queryByLabelText(DESELECT_BUTTON)
    userEvent.click(deselectButton as HTMLButtonElement)
  }
  const clearResources = () => {
    const clearButton = screen.queryByLabelText(CLEAR_BUTTON)
    userEvent.click(clearButton as HTMLButtonElement)
  }

  beforeEach(() => {
    render(
      <ResourceSelectionProvider initialValuesForTesting={INITIAL_SELECTIONS}>
        <TestConsumer />
      </ResourceSelectionProvider>
    )
  })

  describe("`selected` property", () => {
    it("reports an accurate list of selected resources", () => {
      expect(selectedState()).toBe(JSON.stringify(INITIAL_SELECTIONS))

      clearResources()
      expect(selectedState()).toBe(JSON.stringify([]))
    })
  })

  describe("isSelected", () => {
    it("returns `true` when a resource is selected", () => {
      setCurrentResource(INITIAL_SELECTIONS[1])
      expect(isSelected()).toBe("true")
    })

    it("returns `false` when a resource is NOT selected", () => {
      setCurrentResource("sprout")
      expect(isSelected()).toBe("false")
    })
  })

  describe("select", () => {
    it("adds a resource to the list of selections", () => {
      selectResource("cool_beans")
      expect(isSelected()).toBe("true")
      expect(selectedState()).toBe(
        JSON.stringify([...INITIAL_SELECTIONS, "cool_beans"])
      )
    })

    it("adds multiple resources to the list of selections", () => {
      selectResource("cool_beans,super_beans,fancy_beans")
      expect(selectedState()).toBe(
        JSON.stringify([
          ...INITIAL_SELECTIONS,
          "cool_beans",
          "super_beans",
          "fancy_beans",
        ])
      )
    })

    it("does NOT select the same resource twice", () => {
      selectResource(INITIAL_SELECTIONS[0])
      expect(isSelected()).toBe("true")
      expect(selectedState()).toBe(JSON.stringify(INITIAL_SELECTIONS))
    })
  })

  describe("deselect", () => {
    it("removes a resource from the list of selections", () => {
      deselectResource(INITIAL_SELECTIONS[0])
      expect(isSelected()).toBe("false")
      expect(selectedState()).toBe(JSON.stringify(INITIAL_SELECTIONS.slice(1)))
    })

    it("removes multiple resources from the list of selections", () => {
      deselectResource(`${INITIAL_SELECTIONS[0]},${INITIAL_SELECTIONS[1]}`)
      expect(selectedState()).toBe(JSON.stringify([INITIAL_SELECTIONS[2]]))
    })
  })

  describe("clearSelections", () => {
    it("removes all resource selections", () => {
      clearResources()
      expect(selectedState()).toBe(JSON.stringify([]))
    })
  })
})
