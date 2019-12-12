import React from "react"
import { storiesOf } from "@storybook/react"
import HUDLayout from "./HUDLayout"
import styled from "styled-components"
import {Color, Height, SizeUnit} from "./constants"

let Header = styled.header`
  border-right: 1px dashed ${Color.white};
  height: 100px;
  padding: ${SizeUnit(0.5)};
`

let StickyNav = styled.nav`
  background-color: ${Color.white};
  color: ${Color.gray};
  padding: ${SizeUnit(0.25)};
`

let Main = styled.main`
  border-right: 1px dashed ${Color.white};
  border-bottom: 1px dashed ${Color.white};
  padding: ${SizeUnit(0.5)};
  white-space: pre;
`


let mainLorem = `Lorem ipsum dolor sit amet, consectetur adipiscing elit, 
sed do eiusmod tempor incididunt ut 
labore et dolore magna aliqua. Ut enim ad minim veniam, 

quis nostrud exercitation ullamco laboris nisi ut 
aliquip ex ea commodo consequat. Duis aute irure dolor
in reprehenderit in voluptate velit esse cillum 
dolore eu fugiat nulla pariatur. 

Excepteur sint occaecat cupidatat non proident, 
sunt in culpa qui officia deserunt mollit anim id est laborum.

Lorem ipsum dolor sit amet, consectetur adipiscing elit, 
sed do eiusmod tempor incididunt ut 
labore et dolore magna aliqua. Ut enim ad minim veniam, 

quis nostrud exercitation ullamco laboris nisi ut 
aliquip ex ea commodo consequat. Duis aute irure dolor
in reprehenderit in voluptate velit esse cillum 
dolore eu fugiat nulla pariatur. 

Excepteur sint occaecat cupidatat non proident, 
sunt in culpa qui officia deserunt mollit anim id est laborum.

Platea dictumst quisque sagittis purus sit amet volutpat consequat mauris. 
Eleifend mi in nulla posuere sollicitudin aliquam. 
Lorem dolor sed viverra ipsum. Laoreet non curabitur gravida arcu ac tortor dignissim. 

Tortor dignissim convallis aenean et tortor at. 
Aliquam sem et tortor consequat id porta nibh. 
Arcu ac tortor dignissim convallis aenean et tortor at.
`

function layoutDefault() {
  return (
    <HUDLayout
      header={<Header>Header</Header>}
      stickyNav={<StickyNav>Sticky Nav</StickyNav>}
      isSidebarClosed={false}
    >
      <Main>{mainLorem}</Main>
    </HUDLayout>
  )
}

function layoutWithSidebarCollapsed() {
  return (
    <HUDLayout
      header={<Header>Header</Header>}
      stickyNav={<StickyNav>Sticky Nav</StickyNav>}
      isSidebarClosed={true}
    >
      <Main>{mainLorem}</Main>
    </HUDLayout>
  )
}

storiesOf("HUDLayout", module)
  .add("default", layoutDefault)
  .add("sidebar-collapsed", layoutWithSidebarCollapsed)

