import ChevronLeftIcon from "@material-ui/icons/ChevronLeft"
import ChevronRightIcon from "@material-ui/icons/ChevronRight"
import React, {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react"
import { Link, useNavigate } from "react-router-dom"
import styled, { createGlobalStyle } from "styled-components"
import { ApiButtonType, buttonsForComponent } from "./ApiButton"
import { ReactComponent as DetailViewSvg } from "./assets/svg/detail-view-icon.svg"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { usePersistentState } from "./BrowserStorage"
import HeaderBar, { HeaderBarPage } from "./HeaderBar"
import { InstrumentedButton } from "./instrumentedComponents"
import OverviewResourceDetails, {
  useResourceAlerts,
} from "./OverviewResourceDetails"
import OverviewResourceHeader from "./OverviewResourceHeader"
import OverviewTable from "./OverviewTable"
import { OverviewTableBulkActions } from "./OverviewTableBulkActions"
import { OverviewTableDisplayOptions } from "./OverviewTableDisplayOptions"
import { usePathBuilder } from "./PathBuilder"
import { useResourceNav } from "./ResourceNav"
import { ResourceNameFilter } from "./ResourceNameFilter"
import { isTargetEditable } from "./shortcut"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { useStarredResources } from "./StarredResourcesContext"
import {
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
  Width,
  ZIndex,
} from "./style-helpers"
import { ResourceName } from "./types"
import {
  prefersReducedMotion,
  runViewTransition,
  transitionName,
  transitionProps,
  transitionScopeProps,
  TRANSITION_DURATION_MS,
  TRANSITION_EASING,
} from "./viewTransition"
import type { View } from "./webview"

type OverviewTablePaneProps = {
  view: View
  isSocketConnected: boolean
}

// Width of the resource list when the details panel is open.
// The divider between them can be dragged; the panel keeps a minimum width.
const CONDENSED_TABLE_WIDTH = 660
const MIN_TABLE_WIDTH = 200
const MIN_PANEL_WIDTH = 480
const DIVIDER_KEYBOARD_STEP = 16
// Dragging the divider this far left collapses the list.
const COLLAPSE_LIST_WIDTH = 140
// Width of the collapsed list, which only shows the expand button.
const COLLAPSED_LIST_WIDTH = 36

function clampTableWidth(width: number): number {
  const max = Math.max(MIN_TABLE_WIDTH, window.innerWidth - MIN_PANEL_WIDTH)
  return Math.round(Math.min(Math.max(width, MIN_TABLE_WIDTH), max))
}

type PanelPhase = "closed" | "open"

// While collapsing or expanding, the list's width animates and its columns
// drop out (or come back) one by one.
type ListState = "expanded" | "collapsing" | "collapsed" | "expanding"

const DRAWER_TRANSITION_NAME = transitionName("drawer")

// Timing for the open/close transition. Everything moves together.
const TransitionStyle = createGlobalStyle`
  @keyframes tilt-drawer-in {
    from {
      transform: translateX(100%);
    }
  }
  @keyframes tilt-drawer-out {
    to {
      transform: translateX(100%);
    }
  }

  ::view-transition-group(*),
  ::view-transition-old(*),
  ::view-transition-new(*) {
    animation-duration: ${TRANSITION_DURATION_MS}ms;
    animation-timing-function: ${TRANSITION_EASING};
  }

  /* Each piece keeps its natural size (no stretching) and is cut off at its
     new edges, so text never squishes as rows get narrower. Content is
     vertically centered in rows and cells, so the before and after pictures
     line up on their centers. */
  ::view-transition-group(*) {
    overflow: clip;
  }
  ::view-transition-old(*),
  ::view-transition-new(*) {
    width: auto;
    height: auto;
    top: 50%;
    translate: 0 -50%;
  }
  ::view-transition-group(root) {
    overflow: visible;
  }

  /* The details panel slides in and out from the right, over the list. */
  ::view-transition-group(${DRAWER_TRANSITION_NAME}) {
    overflow: visible;
    z-index: 1;
  }
  ::view-transition-new(${DRAWER_TRANSITION_NAME}):only-child {
    animation-name: tilt-drawer-in;
  }
  ::view-transition-old(${DRAWER_TRANSITION_NAME}):only-child {
    animation-name: tilt-drawer-out;
  }
`

let OverviewTablePaneStyle = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.gray20};
`

const OverviewTableStickyNav = styled.div`
  background-color: ${Color.gray20};
  position: sticky;
  top: 0;
  z-index: ${ZIndex.TableStickyHeader};
`

const OverviewTableMenu = styled.section`
  display: flex;
  flex-direction: row;
  align-items: center;
  margin-left: auto;
  margin-right: auto;
  /* Max and min width are based on fixed table layout and column widths */
  max-width: 2000px;
  min-width: 1400px;

  @media screen and (max-width: 2200px) {
    margin-left: ${SizeUnit(1 / 2)};
    margin-right: ${SizeUnit(1 / 2)};
  }
`

const OverviewTableResourceNameFilter = styled(ResourceNameFilter)`
  margin-left: ${SizeUnit(1 / 2)};
  margin-right: ${SizeUnit(1 / 2)};
  min-width: ${Width.sidebarDefault}px;
`

// Holds the resource list and, when a resource is open, the details panel.
const MasterDetail = styled.div`
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  /* "clip" (unlike "hidden") can't be scrolled, so the log pane scrolling
     itself into view can't push the list sideways. */
  overflow: clip;
`

const ListColumn = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
`

// With the details panel open, the list's filter and display options
// sit above the list instead of across the page.
const ListControls = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${SizeUnit(0.15)};
  padding: ${SizeUnit(0.25)} 0 ${SizeUnit(0.15)} ${SizeUnit(0.5)};
`

const ListFilterRow = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.15)};

  & > :first-child {
    flex: 1 1 auto;
    min-width: 0;
  }
`

const ListDisplayOptions = styled(OverviewTableDisplayOptions)`
  margin-left: 0;

  /* While the width animates, keep the options on one line so the list
     below doesn't jump up and down. */
  &.isAnimating {
    flex-wrap: nowrap;
    white-space: nowrap;
  }
`

const ListToggleButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-width: 0;
  padding: ${SizeUnit(0.15)};

  svg {
    fill: ${Color.gray50};
  }

  &:hover svg {
    fill: ${Color.gray70};
  }
`

const CollapsedList = styled.div`
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.15)};
  border-right: 1px solid ${Color.gray40};
  height: 100%;
`

const TableScroll = styled.div`
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;

  &.isHidden {
    display: none;
  }
`

const Divider = styled.div`
  flex: 0 0 ${SizeUnit(1 / 2)};
  position: relative;
  cursor: col-resize;
  touch-action: none;

  &::after {
    content: "";
    position: absolute;
    top: 0;
    bottom: 0;
    right: 0;
    width: 1px;
    background-color: ${Color.gray40};
  }

  &:hover::after,
  &:focus-visible::after,
  &.isDragging::after {
    width: 3px;
    background-color: ${Color.blue};
  }

  &:focus {
    outline: none;
  }
`

// "All Resources" and "Starred Resources": combined logs, shown above the
// resource groups.
const BuiltinRows = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${SizeUnit(0.25)};
  margin-bottom: ${SizeUnit(0.5)};
`

const BuiltinRow = styled(Link)`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.35)};
  padding: ${SizeUnit(0.3)} ${SizeUnit(0.5)};
  border: 1px solid ${Color.gray40};
  border-radius: 4px;
  background-color: ${Color.gray10};
  color: ${Color.offWhite};
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;

  svg {
    fill: ${Color.gray60};
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  &:hover {
    background-color: ${Color.gray30};
  }

  &.isOpen {
    background-color: ${ColorRGBA(Color.blue, ColorAlpha.almostTransparent)};
    border-left: 4px solid ${Color.blue};
    font-weight: bold;
  }
`

const BuiltinRowHint = styled.span`
  color: ${Color.gray60};
  font-size: ${FontSize.smallest};
  font-weight: normal;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
`

const DetailPanel = styled.section`
  flex: 1 1 auto;
  min-width: 0;
  /* Keep the panel window-height, so long logs scroll inside the log pane
     instead of stretching the page (and scrolling the table out of view). */
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background-color: ${Color.gray10};
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.35);
`

function BuiltinResourceRows(props: { openName: string }) {
  const pb = usePathBuilder()
  const { starredResources } = useStarredResources()
  return (
    <BuiltinRows>
      <BuiltinRow
        to={pb.encpath`/r/${ResourceName.all}/overview`}
        className={props.openName === ResourceName.all ? "isOpen" : ""}
        aria-label="View all resource logs"
        {...transitionProps("builtin/all")}
      >
        <DetailViewSvg role="presentation" />
        All Resources
        <BuiltinRowHint>combined logs</BuiltinRowHint>
      </BuiltinRow>
      {starredResources.length > 0 && (
        <BuiltinRow
          to={pb.encpath`/r/${ResourceName.starred}/overview`}
          className={props.openName === ResourceName.starred ? "isOpen" : ""}
          aria-label="View starred resource logs"
          {...transitionProps("builtin/starred")}
        >
          <StarSvg role="presentation" />
          Starred Resources
          <BuiltinRowHint>combined logs</BuiltinRowHint>
        </BuiltinRow>
      )}
    </BuiltinRows>
  )
}

export default function OverviewTablePane(props: OverviewTablePaneProps) {
  const nav = useResourceNav()
  const navigate = useNavigate()
  const pb = usePathBuilder()
  const { starredResources } = useStarredResources()
  const rootRef = useRef<HTMLDivElement>(null)

  // Any resource in the URL opens the details panel: a single resource,
  // all resources, the starred resources, or one that doesn't exist.
  const urlName = nav.invalidResource || nav.selectedResource
  const isOpen = !!urlName

  // While the panel transitions out, keep showing what it showed.
  const lastOpenName = useRef(urlName)
  if (isOpen) {
    lastOpenName.current = urlName
  }
  const name = isOpen ? urlName : lastOpenName.current

  // The layout follows the URL, one step behind, so the change from table
  // to details (and back) can be animated as one transition.
  const [phase, setPhase] = useState<PanelPhase>(isOpen ? "open" : "closed")
  const isCondensed = phase === "open"
  const isOpenRef = useRef(isOpen)
  isOpenRef.current = isOpen
  const transitionPending = useRef(false)
  useLayoutEffect(() => {
    if (transitionPending.current || isCondensed === isOpen) {
      return
    }
    transitionPending.current = true
    runViewTransition(rootRef.current, () => {
      transitionPending.current = false
      setPhase(isOpenRef.current ? "open" : "closed")
    })
  }, [isOpen, isCondensed])

  const resources = props.view.uiResources || []
  const resource = resources.find((r) => r.metadata?.name === name)
  let alertResources = resource ? [resource] : []
  if (name === ResourceName.all) {
    alertResources = resources
  } else if (name === ResourceName.starred) {
    alertResources = resources.filter((r) =>
      starredResources.includes(r.metadata?.name || "")
    )
  }
  const alerts = useResourceAlerts(alertResources)
  const buttons = buttonsForComponent(
    props.view.uiButtons,
    ApiButtonType.Resource,
    name
  )

  const [tableWidth, setTableWidth] = usePersistentState(
    "condensed-table-width",
    CONDENSED_TABLE_WIDTH
  )

  // Collapsing the list gives the logs (almost) the whole width.
  // Like the old sidebar, this isn't saved between visits.
  const [listState, setListState] = useState<ListState>("expanded")
  const isListAnimating =
    listState === "collapsing" || listState === "expanding"
  const isListCollapsed = listState === "collapsed"
  const setListCollapsed = (collapsed: boolean) => {
    const animate = !prefersReducedMotion()
    if (collapsed) {
      setListState(animate ? "collapsing" : "collapsed")
    } else {
      setListState(animate ? "expanding" : "expanded")
    }
  }
  useEffect(() => {
    if (!isListAnimating) {
      return
    }
    const timeout = setTimeout(
      () => setListState(listState === "collapsing" ? "collapsed" : "expanded"),
      TRANSITION_DURATION_MS
    )
    return () => clearTimeout(timeout)
  }, [listState])

  // Track the list's width, so columns can drop out as it narrows
  // (while it animates, or when the divider is dragged).
  const listRef = useRef<HTMLDivElement>(null)
  const [listWidth, setListWidth] = useState<number | undefined>()
  useEffect(() => {
    const list = listRef.current
    if (!list || typeof ResizeObserver === "undefined") {
      return
    }
    const observer = new ResizeObserver(([entry]) =>
      setListWidth(entry.contentRect.width)
    )
    observer.observe(list)
    return () => observer.disconnect()
  }, [])

  const onDividerPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault()
    const divider = e.currentTarget
    const startX = e.clientX
    const startWidth = clampTableWidth(tableWidth)
    divider.setPointerCapture(e.pointerId)
    divider.classList.add("isDragging")
    const onUp = () => {
      divider.classList.remove("isDragging")
      divider.removeEventListener("pointermove", onMove)
      divider.removeEventListener("pointerup", onUp)
      divider.removeEventListener("pointercancel", onUp)
    }
    const onMove = (e: PointerEvent) => {
      const width = startWidth + e.clientX - startX
      if (width < COLLAPSE_LIST_WIDTH) {
        onUp()
        setTableWidth(startWidth)
        setListCollapsed(true)
        return
      }
      setTableWidth(clampTableWidth(width))
    }
    divider.addEventListener("pointermove", onMove)
    divider.addEventListener("pointerup", onUp)
    divider.addEventListener("pointercancel", onUp)
  }

  const onDividerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      const step =
        e.key === "ArrowLeft" ? -DIVIDER_KEYBOARD_STEP : DIVIDER_KEYBOARD_STEP
      setTableWidth(clampTableWidth(tableWidth + step))
      e.preventDefault()
    }
  }

  const closeDetails = useCallback(
    () => navigate(pb.encpath`/overview`),
    [navigate, pb]
  )

  // Esc closes the panel. If you're typing in a field (like the log filter),
  // the first Esc just leaves the field; the next one closes the panel.
  useEffect(() => {
    if (!isOpen) {
      return
    }
    const onKeydown = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || e.defaultPrevented) {
        return
      }
      if (isTargetEditable(e)) {
        ;(e.target as HTMLElement).blur()
        return
      }
      closeDetails()
    }
    document.body.addEventListener("keydown", onKeydown)
    return () => document.body.removeEventListener("keydown", onKeydown)
  }, [isOpen, closeDetails])

  const expandedListWidth = clampTableWidth(tableWidth)
  let listStyle: React.CSSProperties = {}
  if (isCondensed) {
    const width =
      listState === "collapsing" || isListCollapsed
        ? COLLAPSED_LIST_WIDTH
        : expandedListWidth
    listStyle = {
      flex: `0 0 ${width}px`,
      transition: isListAnimating
        ? `flex-basis ${TRANSITION_DURATION_MS}ms ${TRANSITION_EASING}`
        : undefined,
    }
  }

  return (
    <OverviewTablePaneStyle ref={rootRef}>
      <TransitionStyle />
      <OverviewTableStickyNav>
        <HeaderBar
          view={props.view}
          currentPage={isOpen ? HeaderBarPage.Detail : HeaderBarPage.Grid}
          isSocketConnected={props.isSocketConnected}
        />
        <StarredResourceBar
          {...starredResourcePropsFromView(props.view, isOpen ? name : "")}
        />
        {!isCondensed && (
          <OverviewTableMenu aria-label="Resource menu">
            <OverviewTableResourceNameFilter {...transitionProps("filter")} />
            <OverviewTableBulkActions uiButtons={props.view.uiButtons} />
            <OverviewTableDisplayOptions
              resources={props.view.uiResources}
              {...transitionProps("display-options")}
            />
          </OverviewTableMenu>
        )}
      </OverviewTableStickyNav>
      <MasterDetail>
        <ListColumn style={listStyle} ref={listRef}>
          {isCondensed && isListCollapsed && (
            <CollapsedList>
              <ListToggleButton
                aria-label="Expand resource list"
                title="Expand resource list"
                onClick={() => setListCollapsed(false)}
              >
                <ChevronRightIcon role="presentation" />
              </ListToggleButton>
            </CollapsedList>
          )}
          {isCondensed && !isListCollapsed && (
            <ListControls aria-label="Resource menu">
              <ListFilterRow>
                <ResourceNameFilter {...transitionProps("filter")} />
                <ListToggleButton
                  aria-label="Collapse resource list"
                  title="Collapse resource list"
                  onClick={() => setListCollapsed(true)}
                >
                  <ChevronLeftIcon role="presentation" />
                </ListToggleButton>
              </ListFilterRow>
              <ListDisplayOptions
                resources={props.view.uiResources}
                className={isListAnimating ? "isAnimating" : ""}
                {...transitionProps("display-options")}
              />
            </ListControls>
          )}
          <TableScroll
            className={isCondensed && isListCollapsed ? "isHidden" : ""}
            {...transitionScopeProps}
          >
            <OverviewTable
              view={props.view}
              condensed={isCondensed}
              availableWidth={listWidth ?? expandedListWidth}
              beforeTables={
                <BuiltinResourceRows openName={isOpen ? name : ""} />
              }
            />
          </TableScroll>
        </ListColumn>
        {isCondensed && !isListCollapsed && (
          <Divider
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize resource list"
            aria-valuenow={tableWidth}
            tabIndex={0}
            title="Drag to resize · double-click to reset"
            onPointerDown={onDividerPointerDown}
            onKeyDown={onDividerKeyDown}
            onDoubleClick={() => setTableWidth(CONDENSED_TABLE_WIDTH)}
          />
        )}
        {isCondensed && (
          <DetailPanel
            aria-label={`Details for ${name}`}
            {...transitionProps("drawer")}
          >
            <OverviewResourceHeader
              name={name}
              resource={resource}
              buttons={buttons}
              allButtons={props.view.uiButtons}
              onClose={closeDetails}
            />
            <OverviewResourceDetails
              resource={resource}
              name={name}
              alerts={alerts}
              buttons={buttons}
              logControlsOnly={true}
            />
          </DetailPanel>
        )}
      </MasterDetail>
    </OverviewTablePaneStyle>
  )
}
