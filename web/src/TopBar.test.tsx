import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"
import TopBar from "./TopBar"

it("shows sail share button", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Errors}
          sailEnabled={true}
          sailUrl=""
          numberOfErrors={0}
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
          sailEnabled={true}
          sailUrl="www.sail.dev/xyz"
          numberOfErrors={1}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
