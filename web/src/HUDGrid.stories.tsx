import React from "react"
import { storiesOf } from "@storybook/react"
import HUDGrid from "./HUDGrid"
import styled from "styled-components"
import { Height } from "./constants"

let TopBar = styled.div`
  height: ${Height.topBar}px
  background-color: #000033;
  display: flex;
  align-items: center;
  padding: 0 16px;
`

let ResourceBar = styled.div`
  height: ${Height.resourceBar}px
  background-color: #000066;
  display: flex;
  align-items: center;
  padding: 0 16px;
`

let Main = styled.div`
  padding: 16px;
  background-color: #000099;
  width: 100%;
`

function gridDefault() {
  return (
    <HUDGrid
      topBar={<TopBar>top bar</TopBar>}
      resourceBar={null}
      isSidebarClosed={false}
    >
      <Main>Hello world</Main>
    </HUDGrid>
  )
}

function gridWithResourceBar() {
  return (
    <HUDGrid
      topBar={<TopBar>top bar</TopBar>}
      resourceBar={<ResourceBar>resource bar</ResourceBar>}
      isSidebarClosed={false}
    >
      <Main>Hello world</Main>
    </HUDGrid>
  )
}

function gridWithSidebarClosed() {
  return (
    <HUDGrid
      topBar={<TopBar>top bar</TopBar>}
      resourceBar={<ResourceBar>resource bar</ResourceBar>}
      isSidebarClosed={true}
    >
      <Main>Hello world</Main>
    </HUDGrid>
  )
}

storiesOf("HUDGrid", module)
  .add("default", gridDefault)
  .add("with-resource-bar", gridWithResourceBar)
  .add("with-sidebar-closed", gridWithSidebarClosed)
