import React from "react"
import { storiesOf } from "@storybook/react"
import HUDLayout from "./HUDLayout"
import styled from "styled-components"
import { Color, Height, SizeUnit } from "./style-helpers"

let Header = styled.header`
  border-right: 1px dashed ${Color.white};
  height: 100px;
  padding: ${SizeUnit(0.5)};
`

let Main = styled.main`
  border-right: 1px dashed ${Color.white};
  border-bottom: 1px dashed ${Color.white};
  padding: ${SizeUnit(0.5)};
`

let mainLorem = (
  <p>
    Lorem ipsum dolor sit amet, consectetur adipiscing elit,
    <br />
    sed do eiusmod tempor incididunt ut
    <br />
    labore et dolore magna aliqua. Ut enim ad minim veniam,
    <br />
    <br />
    quis nostrud exercitation ullamco laboris nisi ut
    <br />
    aliquip ex ea commodo consequat. Duis aute irure dolor
    <br />
    in reprehenderit in voluptate velit esse cillum
    <br />
    dolore eu fugiat nulla pariatur.
    <br />
    <div style={{ textAlign: "right" }}>Initial Build - blog-site</div>
    <br />
    Excepteur sint occaecat cupidatat non proident,
    <br />
    sunt in culpa qui officia deserunt mollit anim id est laborum.
    <br />
    <br />
    Lorem ipsum dolor sit amet, consectetur adipiscing elit,
    <br />
    sed do eiusmod tempor incididunt ut
    <br />
    labore et dolore magna aliqua. Ut enim ad minim veniam,
    <br />
    <br />
    quis nostrud exercitation ullamco laboris nisi ut
    <br />
    aliquip ex ea commodo consequat. Duis aute irure dolor
    <br />
    in reprehenderit in voluptate velit esse cillum
    <br />
    dolore eu fugiat nulla pariatur.
    <br />
    <br />
    Excepteur sint occaecat cupidatat non proident,
    <br />
    sunt in culpa qui officia deserunt mollit anim id est laborum.
    <br />
    <br />
    Platea dictumst quisque sagittis purus sit amet volutpat consequat mauris.
    <br />
    Eleifend mi in nulla posuere sollicitudin aliquam.
    <br />
    Lorem dolor sed viverra ipsum. Laoreet non curabitur gravida arcu ac tortor
    dignissim.
    <br />
    <br />
    Tortor dignissim convallis aenean et tortor at.
    <br />
    Aliquam sem et tortor consequat id porta nibh.
    <br />
    Arcu ac tortor dignissim convallis aenean et tortor at.
    <br />
  </p>
)

function layoutDefault() {
  return (
    <HUDLayout header={<Header>Header</Header>} isSidebarClosed={false}>
      <Main>{mainLorem}</Main>
    </HUDLayout>
  )
}

function layoutWithSidebarCollapsed() {
  return (
    <HUDLayout header={<Header>Header</Header>} isSidebarClosed={true}>
      <Main>{mainLorem}</Main>
    </HUDLayout>
  )
}

function layoutTwoLevelNav() {
  return (
    <HUDLayout
      header={<Header>Header</Header>}
      isSidebarClosed={false}
      isTwoLevelHeader={true}
    >
      <Main>{mainLorem}</Main>
    </HUDLayout>
  )
}

storiesOf("HUDLayout", module)
  .add("default", layoutDefault)
  .add("sidebar-collapsed", layoutWithSidebarCollapsed)
  .add("two-level-nav", layoutTwoLevelNav)
