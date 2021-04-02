import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import StarredResourceBar, {
  StarButton,
  StarredResource,
  StarredResourceLabel,
} from "./StarredResourceBar"
import { ResourceStatus } from "./types"

const pathBuilder = PathBuilder.forTesting("localhost", "/")

function getStarredItemNames(
  root: ReactWrapper<any, React.Component["state"], React.Component>
): Array<string> {
  let starredItems = root.find(StarredResourceLabel)
  return starredItems.map((i) => i.text())
}

function clickStar(
  root: ReactWrapper<any, React.Component["state"], React.Component>,
  name: string
) {
  const r = root
    .find(StarredResource)
    .find({ resource: { name: name } })
    .find(StarButton)
  expect(r.length).toEqual(1)
  r.at(0).simulate("click")
}

it("renders the starred items", () => {
  const resources = [
    { name: "foo", status: ResourceStatus.Healthy },
    { name: "bar", status: ResourceStatus.Unhealthy },
  ]

  const root = mount(
    <MemoryRouter>
      <StarredResourceBar resources={resources} unstar={() => {}} />
    </MemoryRouter>
  )

  expect(getStarredItemNames(root)).toEqual(["foo", "bar"])
})

it("calls unstar", () => {
  const resources = [
    { name: "foo", status: ResourceStatus.Healthy },
    { name: "bar", status: ResourceStatus.Unhealthy },
  ]

  let unstars: string[] = []
  let onClick = (resourceName: string) => {
    unstars.push(resourceName)
  }

  const root = mount(
    <MemoryRouter>
      <StarredResourceBar resources={resources} unstar={onClick} />
    </MemoryRouter>
  )

  clickStar(root, "foo")

  expect(unstars).toEqual(["foo"])
})
