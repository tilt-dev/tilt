import React from "react"
import renderer from "react-test-renderer"
import TabNav from "./TabNav"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"

it("shows logs", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Log}
          sailUrl=""
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("previews resources", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Preview}
          sailUrl=""
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows error pane", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          errorsUrl="/r/foo/errors"
          resourceView={ResourceView.Errors}
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
                <TabNav
                    logUrl="/r/foo"
                    previewUrl="/r/foo/preview"
                    errorsUrl="/r/foo/errors"
                    resourceView={ResourceView.Errors}
                    sailUrl="www.sail.dev/xyz"
                />
            </MemoryRouter>
        )
        .toJSON()

    expect(tree).toMatchSnapshot()
})
