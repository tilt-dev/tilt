import React from "react"
import PreviewList from "./PreviewList"
import renderer from "react-test-renderer"
import PathBuilder from "./PathBuilder"
import { MemoryRouter } from "react-router"

it("renders a message if no resources have endpoint", () => {
  let pb = new PathBuilder("", "")

  const tree = renderer
    .create(<PreviewList resourcesWithEndpoints={[]} pathBuilder={pb} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders a list of resources that have endpoints", () => {
  let pb = new PathBuilder("", "")

  const tree = renderer
    .create(
      <MemoryRouter initialEntries={["/"]}>
        <PreviewList
          resourcesWithEndpoints={["snack", "vigoda"]}
          pathBuilder={pb}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
