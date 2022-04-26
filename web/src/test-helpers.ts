import { Component } from "react"
import { render } from "react-dom"

/**
 * Generic test helper functions
 */

/**
 * There are a couple places in our tests where we rely on asserting on
 * and manipulating React component state directly. This isn't possible
 * to do with React Testing Library. Instead, we use React's testing utils
 * (with minor Typescript gymnastics) to get the component instance and
 * access the DOM.
 */
export function renderTestComponent<T>(component: JSX.Element) {
  const container = document.createElement("div")
  const rootTree = render<T>(component, container)

  if (isComponent<T>(rootTree)) {
    return { container, rootTree }
  } else {
    throw new Error("React render did not return a component")
  }
}

// A rudimentary helper function to type the output of `render()` properly
function isComponent<T>(
  renderOutput: void | Element | Component<T>
): renderOutput is Component<T> {
  return !!renderOutput
}
