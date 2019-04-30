import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"
import TopBar from "./TopBar"
import PathBuilder from "./PathBuilder"

let pathBuilder = new PathBuilder("localhost", "/")

it("shows sail share button", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Errors}
          pathBuilder={pathBuilder}
          sailEnabled={true}
          sailUrl=""
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows sail url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Errors}
          pathBuilder={pathBuilder}
          sailEnabled={true}
          sailUrl="www.sail.dev/xyz"
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
