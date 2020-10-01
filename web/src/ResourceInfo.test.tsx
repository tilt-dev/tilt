import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView, SnapshotHighlight, TriggerMode } from "./types"
import ResourceInfo from "./ResourceInfo"
import fetchMock from "jest-fetch-mock"
import { mount } from "enzyme"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { unnamedEndpointLink, namedEndpointLink } from "./testdata"

const fakeHandleOpenModal = () => {}

it("shows snapshot url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <ResourceInfo
          showSnapshotButton={true}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("doesn't render snapshot button if it's a snapshot", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <ResourceInfo
          showSnapshotButton={false}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("uses endpoint name as endpoint text, if given", () => {
  const root = mount(
    <ResourceInfo
      showSnapshotButton={false}
      handleOpenModal={fakeHandleOpenModal}
      highlight={null}
      endpoints={[namedEndpointLink]}
    />
  )

  let links = root.find("span#endpoints a")
  expect(links).toHaveLength(1)
  expect(links.at(0).prop("href")).toEqual(namedEndpointLink.url)
  expect(links.at(0).text()).toEqual(namedEndpointLink.name)
})

it("uses url as endpoint text if name not given", () => {
  const root = mount(
    <ResourceInfo
      showSnapshotButton={false}
      handleOpenModal={fakeHandleOpenModal}
      highlight={null}
      endpoints={[unnamedEndpointLink]}
    />
  )

  let links = root.find("span#endpoints a")
  expect(links).toHaveLength(1)
  expect(links.at(0).prop("href")).toEqual(unnamedEndpointLink.url)
  expect(links.at(0).text()).toEqual(unnamedEndpointLink.url)
})

it("displays mixed named/unnamed endpoints", () => {
  const root = mount(
    <ResourceInfo
      showSnapshotButton={false}
      handleOpenModal={fakeHandleOpenModal}
      highlight={null}
      endpoints={[unnamedEndpointLink, namedEndpointLink]}
    />
  )

  let links = root.find("span#endpoints a")
  expect(links).toHaveLength(2)

  expect(links.at(0).prop("href")).toEqual(unnamedEndpointLink.url)
  expect(links.at(0).text()).toEqual(unnamedEndpointLink.url)

  expect(links.at(1).prop("href")).toEqual(namedEndpointLink.url)
  expect(links.at(1).text()).toEqual(namedEndpointLink.name)
})
