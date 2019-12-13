import React from "react"
import styled from "styled-components"
import { AnimDuration, Height, Width, ZIndex } from "./constants"

// The HUD UI looks like this:
//
//            +----------------------------+---------+
//            | Header                     | Sidebar |
//            |                            |         |
//            +----------------------------+         |
//  StickyNav +----------------------------+         |
//            |                            |         |
//            | Main                       |         |
//            |                            |         |
//            |                            |         |
//            |                            |         |
//            |                            |         |
//            +--------------------------------------+
//            +--------------------------------------+ Statusbar
//
// We need to satisfy several constraints:
//
// 1) Main streams logs and can grow very tall.
//    So we expect scrolling to be a common interaction.
//
// 2) Sidebar abuts Main, and is collapsible.
//    Sidebar never covers any content within HUDLayout.
//
// 3) StickyNav does not scroll offscreen, but sticks to the top.
//
// 4) Header, StickyNav, and Statusbar may temporarily cover Main content,
//    but scrolling should make any covered content visible.
//
//
// To create this layout:
//
//    We're avoiding the approach of making Main `overflow: auto / scroll-y`.
//    Inner-div-scrolls can have lots of accessibility and UX issues.
//    (Nick can tell you hours of stories about this. e.g.,
//    https://medium.engineering/the-case-of-the-eternal-blur-ab350b9653ea)
//
//    HUDLayout has side padding that dynamically matches the Sidebar width,
//    and bottom padding that matches Statusbar height. So when Sidebar
//    and Statusbar are put in place with `position: fixed`, nothing is covered.
//
//    This way, scrolling anywhere on the page will scroll Main content.
//    (Unless you scroll atop the Sidebar, which _is_ `overflow: auto` 👀)

type HUDLayoutProps = {
  header: React.ReactNode
  stickyNav: React.ReactNode

  // The main pane
  children: React.ReactNode

  isSidebarClosed: boolean
}

let Root = styled.div`
  display: flex;
  flex-direction: column;
  padding-right: ${Width.sidebar}px;
  padding-bottom: ${Height.statusbar}px;
  transition: padding-right ${AnimDuration.default} ease;

  &.is-sidebarCollapsed {
    padding-right: ${Width.sidebarCollapsed}px;
  }
`

let Header = styled.header``

let StickyNav = styled.nav`
  position: sticky;
  top: 0;
`

let Main = styled.main``

export default function HUDLayout(props: HUDLayoutProps) {
  return (
    <Root className={props.isSidebarClosed ? "is-sidebarCollapsed" : ""}>
      <Header>{props.header}</Header>
      <StickyNav>{props.stickyNav}</StickyNav>
      <Main>{props.children}</Main>
    </Root>
  )
}
