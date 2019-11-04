import React from "react"
import styled from "styled-components"
import { Height, Width, ZIndex } from "./constants"

// The HUD UI looks like this:
//
// ----------------------------------
// | top bar                    | S |
// -----------------------------| i |
// | resource info bar          | d |
// -----------------------------| e |
// | main pane                  | b |
// |                            | a |
// |                            | r |
// |                            |   |
// |                            |   |
// |                            |   |
// ----------------------------------
// | status bar                     |
// ----------------------------------
//
// Our DOM architecture needs to satisfy two competing constraints:
//
// 1) "window scroll" needs to scroll the main pane.
//    Inner-div-scrolls generally have lots of accessibility/UX problems
//    Nick can tell you hours of stories about this.
//    https://medium.engineering/the-case-of-the-eternal-blur-ab350b9653ea
//
// 2) The topbar / sidebar / statusbar need to visually frame the main pane,
//    like normal UI panels. Stuff shouldn't get stuck under them.
//
// To create this layout, we make the topbar, sidebar, and statusbar position: fixed.
// Then we create a ~ShadowGrid~ underneath them to preserve margins.

type HUDGridProps = {
  topBar: React.ReactNode
  resourceBar: React.ReactNode

  // The main pane
  children: React.ReactNode

  isSidebarClosed: boolean
}

let Root = styled.div`
  display: flex;
  align-items: stretch;
  width: 100%;
`

let TopFrame = styled.div`
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  transition: padding-right 300ms ease;
  z-index: ${ZIndex.topFrame};
  padding-right: ${Width.sidebar}px;
  height: ${Height.topBar}px;

  ${Root}.is-resourcebarvisible & {
    height: ${Height.topBar + Height.resourceBar}px;
  }

  ${Root}.is-sidebarclosed & {
    padding-right: ${Width.sidebarCollapsed}px;
  }
`

let MainFrame = styled.div`
  display: flex;
  align-items: stretch;
  width: 100%;
  padding-bottom: ${Height.statusbar}px;
  transition: padding-right 300ms ease;
  padding-top: ${Height.topBar}px;
  padding-right: ${Width.sidebar}px;

  ${Root}.is-resourcebarvisible & {
    padding-top: ${Height.topBar + Height.resourceBar}px;
  }

  ${Root}.is-sidebarclosed & {
    padding-right: ${Width.sidebarCollapsed}px;
  }
`

export default function HUDGrid(props: HUDGridProps) {
  let classes = []
  if (props.isSidebarClosed) {
    classes.push("is-sidebarclosed")
  }
  if (props.resourceBar) {
    classes.push("is-resourcebarvisible")
  }
  return (
    <Root className={classes.join(" ")}>
      <TopFrame>
        {props.topBar}
        {props.resourceBar}
      </TopFrame>
      <MainFrame>{props.children}</MainFrame>
    </Root>
  )
}
