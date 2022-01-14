import { mount, ReactWrapper } from "enzyme"
import React, { ChangeEvent, useState } from "react"
import {
  ResourceSelectionProvider,
  useResourceSelection,
} from "./ResourceSelectionContext"

const SELECTED_STATE_ID = "selected-state"
const SELECTED_TEXT_ID = "selected-text"
const SELECT_INPUT_ID = "select-input"
const SELECT_BUTTON_ID = "select-button"
const DESELECT_BUTTON_ID = "deselect-button"
const CLEAR_BUTTON_ID = "clear-button"

// This is a very basic test component that prints out the state
// from the ResourceSelection context and provides buttons to trigger
// methods returned by the context, so they can be tested
const TestConsumer = () => {
  const { selected, isSelected, select, deselect, clearSelections } =
    useResourceSelection()

  const [value, setValue] = useState("")
  const onChange = (e: ChangeEvent<HTMLInputElement>) =>
    setValue(e.target.value)
  return (
    <>
      {/* Print the `selected` state */}
      <p id={SELECTED_STATE_ID}>{JSON.stringify(selected)}</p>
      {/* Use an input field to change the resource that can be selected/deselected */}
      <input id={SELECT_INPUT_ID} type="text" onChange={onChange} />
      {/* Print the state for whatever resource is currently in the `input` */}
      <p id={SELECTED_TEXT_ID}>{isSelected(value).toString()}</p>
      {/* Select the resource that's currently in the `input` */}
      <button id={SELECT_BUTTON_ID} onClick={() => select(value)}>
        Select
      </button>
      {/* Deselect the resource that's currently in the `input` */}
      <button id={DESELECT_BUTTON_ID} onClick={() => deselect(value)}>
        Deselect
      </button>
      {/* Clear all selections */}
      <button id={CLEAR_BUTTON_ID} onClick={() => clearSelections()}>
        Clear selections
      </button>
    </>
  )
}

describe("ResourceSelectionContext", () => {
  let wrapper: ReactWrapper<typeof TestConsumer>

  // Helpers
  const INITIAL_SELECTIONS = ["vigoda", "magic_beans"]
  const selectedState = () => wrapper.find(`#${SELECTED_STATE_ID}`).text()
  const isSelected = () => wrapper.find(`#${SELECTED_TEXT_ID}`).text()
  const setCurrentResource = (value: string) =>
    wrapper
      .find(`#${SELECT_INPUT_ID}`)
      .simulate("change", { target: { value } })
  const selectResource = (value: string) => {
    setCurrentResource(value)
    wrapper.find(`#${SELECT_BUTTON_ID}`).simulate("click")
    wrapper.update()
  }
  const deselectResource = (value: string) => {
    setCurrentResource(value)
    wrapper.find(`#${DESELECT_BUTTON_ID}`).simulate("click")
    wrapper.update()
  }
  const clearResources = () => {
    wrapper.find(`#${CLEAR_BUTTON_ID}`).simulate("click")
    wrapper.update()
  }

  beforeEach(() => {
    wrapper = mount(
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
  })

  describe("deselect", () => {
    it("removes a resource to the list of selections", () => {
      deselectResource(INITIAL_SELECTIONS[0])
      expect(isSelected()).toBe("false")
      expect(selectedState()).toBe(JSON.stringify([INITIAL_SELECTIONS[1]]))
    })
  })

  describe("clearSelections", () => {
    it("removes all resource selections", () => {
      clearResources()
      expect(selectedState()).toBe(JSON.stringify([]))
    })
  })
})
