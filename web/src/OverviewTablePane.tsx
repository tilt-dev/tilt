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
import { usePersistentState, useSessionState } from "./BrowserStorage"
import HeaderBar, { HeaderBarPage } from "./HeaderBar"
import { InstrumentedButton } from "./instrumentedComponents"
import { LastOpenedResourceProvider } from "./LastOpenedResource"
import OverviewResourceDetails, {
  useResourceAlerts,
} from "./OverviewResourceDetails"
import OverviewResourceHeader from "./OverviewResourceHeader"
import OverviewTable from "./OverviewTable"
import {
  CONDENSED_SNAP_WIDTHS,
  FULL_CONDENSED_WIDTH,
  hiddenCondensedColumns,
} from "./OverviewTableColumns"
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
  ColorRGB,
} from "./style-helpers"
import { useLogAlertIndex } from "./LogStore"
import { buildStatus, runtimeStatus } from "./status"
import { ResourceName, ResourceStatus } from "./types"
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

// Width of the resource list when the details panel is open: by default,
// wide enough for all its columns. The divider between them can be dragged;
// below that width it snaps to where whole columns fit, so the list never
// shows half a column. The panel keeps a minimum width.
// A little extra over the full width gives longer names room.
const CONDENSED_TABLE_WIDTH = FULL_CONDENSED_WIDTH + 56
// Windows at least this wide start with every column in the list.
const ROOMY_WINDOW_WIDTH = 1680

// How wide the list starts. On a laptop, just wide enough for the star,
// trigger, name and status, so the logs get most of the room ("Updated"
// is one click away, on the › next to ‹). Wider windows have room for all.
export function defaultTableWidth(windowWidth = window.innerWidth): number {
  return windowWidth >= ROOMY_WINDOW_WIDTH
    ? CONDENSED_TABLE_WIDTH
    : CONDENSED_SNAP_WIDTHS[2]
}
const MIN_TABLE_WIDTH = CONDENSED_SNAP_WIDTHS[0]
const MIN_PANEL_WIDTH = 480
const DIVIDER_KEYBOARD_STEP = 16
const SNAP_DURATION_MS = 150
// While dragging, the divider only moves to another step once the pointer
// is this much closer to it, so it doesn't flicker between two steps.
const SNAP_HYSTERESIS = 24
// Dragging the divider this far left collapses the list.
const COLLAPSE_LIST_WIDTH = 140
// After › widens the list, the full-table › ignores clicks for this long.
const FULL_TABLE_CLICK_GUARD_MS = 600
// Width of the collapsed list, which only shows the expand button.
const COLLAPSED_LIST_WIDTH = 36

function clampTableWidth(width: number): number {
  const max = Math.max(MIN_TABLE_WIDTH, window.innerWidth - MIN_PANEL_WIDTH)
  return Math.round(Math.min(Math.max(width, MIN_TABLE_WIDTH), max))
}

// Below the full width, round to the nearest width where whole columns fit.
function snapTableWidth(width: number): number {
  if (width >= FULL_CONDENSED_WIDTH) {
    return width
  }
  return CONDENSED_SNAP_WIDTHS.reduce((best, w) =>
    Math.abs(w - width) < Math.abs(best - width) ? w : best
  )
}

// Where the divider goes while dragging: the step nearest the pointer, but
// it stays on its current step until the pointer is clearly past halfway.
export function snapWhileDragging(
  pointerWidth: number,
  current: number
): number {
  const target = snapTableWidth(pointerWidth)
  if (
    target === current ||
    pointerWidth >= FULL_CONDENSED_WIDTH ||
    current >= FULL_CONDENSED_WIDTH
  ) {
    return target
  }
  const isClearlyCloser =
    Math.abs(pointerWidth - target) + SNAP_HYSTERESIS <
    Math.abs(pointerWidth - current)
  return isClearlyCloser ? target : current
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

// Always two lines, at any list width, so the list below never shifts up
// or down as the divider moves:
//   [ ] Show disabled resources
//   [ ] Alerts on top          ⇱ ⇲
const ListDisplayOptions = styled(OverviewTableDisplayOptions)`
  margin-left: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  white-space: nowrap;

  & > :first-child {
    grid-column: 1 / -1;
  }

  & > label {
    min-width: 0;
    overflow: hidden;
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

const BackToDetailsButton = styled(ListToggleButton)`
  margin-left: ${SizeUnit(0.25)};
`

const CollapsedList = styled.div`
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.15)};
  border-right: 1px solid ${Color.gray40};
  height: 100%;
  box-sizing: border-box;
`

// While the list is collapsed: one red dot per resource with an error,
// so you can tell something else broke without opening the list.
const ErrorRail = styled.ul`
  list-style: none;
  margin: ${SizeUnit(0.35)} 0 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: ${SizeUnit(0.2)};
  min-height: 0;
  overflow-y: auto;
`

// The collapsed strip's right edge: drag it right to open the list.
const CollapsedEdge = styled.div`
  position: absolute;
  top: 0;
  bottom: 0;
  right: -5px;
  width: 9px;
  cursor: col-resize;
  touch-action: none;
  z-index: 1;

  &::after {
    content: "";
    position: absolute;
    top: 0;
    bottom: 0;
    left: 4px;
    width: 1px;
  }

  &:hover::after,
  &.isDragging::after {
    left: 3px;
    width: 3px;
    background-color: ${Color.blue};
  }
`

const ErrorDot = styled(Link)`
  display: block;
  width: 10px;
  height: 10px;
  margin: 3px;
  border-radius: 50%;
  background-color: ${Color.red};
  opacity: 0.85;

  &:hover,
  &:focus-visible {
    opacity: 1;
    box-shadow: 0 0 0 3px ${ColorRGBA(ColorRGB.red, ColorAlpha.translucent)};
  }

  &.isOpen {
    box-shadow: 0 0 0 2px ${Color.gray10}, 0 0 0 3px ${Color.red};
  }
`

function CollapsedErrorRail(props: { view: View; openName: string }) {
  const pb = usePathBuilder()
  const alertIndex = useLogAlertIndex()
  const broken = (props.view.uiResources || [])
    .filter(
      (r) =>
        buildStatus(r, alertIndex) === ResourceStatus.Unhealthy ||
        runtimeStatus(r, alertIndex) === ResourceStatus.Unhealthy
    )
    .map((r) => r.metadata?.name || "")
    .filter((name) => !!name)
  if (broken.length === 0) {
    return null
  }
  return (
    <ErrorRail aria-label="Resources with errors">
      {broken.map((name) => (
        <li key={name}>
          <ErrorDot
            to={pb.encpath`/r/${name}/overview`}
            className={name === props.openName ? "isOpen" : ""}
            aria-label={`${name}: error`}
            title={`${name}: error`}
          />
        </li>
      ))}
    </ErrorRail>
  )
}

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
    background-color: ${ColorRGBA(ColorRGB.blue, ColorAlpha.almostTransparent)};
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

  // Remember what was open, so the Detail view button can bring it back.
  // If it's gone since (or nothing was opened yet), that's all resources.
  const [lastOpened, setLastOpened] = useSessionState(
    "last-opened-resource",
    ""
  )
  const openedName = isOpen && !nav.invalidResource ? urlName : ""
  useEffect(() => {
    if (openedName && openedName !== lastOpened) {
      setLastOpened(openedName)
    }
  }, [openedName])
  const lastOpenedExists =
    lastOpened === ResourceName.all ||
    (lastOpened === ResourceName.starred && starredResources.length > 0) ||
    resources.some((r) => r.metadata?.name === lastOpened)
  const detailResource = lastOpenedExists ? lastOpened : ResourceName.all
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

  // Only a width you chose (by dragging) is remembered; until then, the
  // list starts at a width that suits the window.
  const [savedTableWidth, setTableWidth] = usePersistentState<number | null>(
    "condensed-list-width",
    null
  )
  const tableWidth = savedTableWidth ?? defaultTableWidth()

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

  // Below the full width, the divider moves in steps, one per layout, so
  // the list changes once per step instead of flickering as it's dragged.
  // Past the full width, it follows the pointer.
  const [isDragging, setIsDragging] = useState(false)
  const [isSnapping, setIsSnapping] = useState(false)
  useEffect(() => {
    if (!isSnapping) {
      return
    }
    const timeout = setTimeout(() => setIsSnapping(false), SNAP_DURATION_MS)
    return () => clearTimeout(timeout)
  }, [isSnapping])
  const glideToWidth = (width: number) => {
    setIsSnapping(!prefersReducedMotion())
    setTableWidth(width)
  }

  // Dragging the divider. Pulling it far enough left collapses the list;
  // pulling it back right (in the same drag, or later from the collapsed
  // strip's edge) opens it again, snapping to whole columns as usual.
  const startListDrag = (
    e: React.PointerEvent<HTMLDivElement>,
    fromCollapsed: boolean
  ) => {
    e.preventDefault()
    const startX = e.clientX
    const startWidth = fromCollapsed
      ? COLLAPSED_LIST_WIDTH
      : clampTableWidth(snapTableWidth(tableWidth))
    let lastWidth = startWidth
    let collapsed = fromCollapsed
    setIsDragging(true)
    const onUp = () => {
      setIsDragging(false)
      window.removeEventListener("pointermove", onMove)
      window.removeEventListener("pointerup", onUp)
      window.removeEventListener("pointercancel", onUp)
    }
    const onMove = (e: PointerEvent) => {
      const width = startWidth + e.clientX - startX
      if (collapsed) {
        // Reopen once the pointer is clearly past the collapse point.
        if (width < COLLAPSE_LIST_WIDTH + SNAP_HYSTERESIS) {
          return
        }
        collapsed = false
        lastWidth = clampTableWidth(snapTableWidth(width))
        setTableWidth(lastWidth)
        setListState("expanded")
        return
      }
      if (width < COLLAPSE_LIST_WIDTH) {
        collapsed = true
        setListCollapsed(true)
        return
      }
      const next = clampTableWidth(snapWhileDragging(width, lastWidth))
      if (next !== lastWidth) {
        lastWidth = next
        setTableWidth(next)
      }
    }
    window.addEventListener("pointermove", onMove)
    window.addEventListener("pointerup", onUp)
    window.addEventListener("pointercancel", onUp)
  }

  // Arrow keys step between the widths where whole columns fit, then
  // (past the full width) in small steps.
  const onDividerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") {
      return
    }
    e.preventDefault()
    const current = snapTableWidth(tableWidth)
    let next: number
    if (e.key === "ArrowRight") {
      next =
        current >= FULL_CONDENSED_WIDTH
          ? current + DIVIDER_KEYBOARD_STEP
          : CONDENSED_SNAP_WIDTHS.find((w) => w > current)!
    } else if (current > FULL_CONDENSED_WIDTH) {
      next = Math.max(current - DIVIDER_KEYBOARD_STEP, FULL_CONDENSED_WIDTH)
    } else {
      next =
        [...CONDENSED_SNAP_WIDTHS].reverse().find((w) => w < current) ?? current
    }
    glideToWidth(clampTableWidth(next))
  }

  // In the full table, ‹ at the far right of the menu goes back to the details
  // (the resource you had open, else all resources), like the Detail icon.
  const detailLabel =
    detailResource === ResourceName.all
      ? "All Resources"
      : detailResource === ResourceName.starred
      ? "Starred Resources"
      : detailResource

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

  const expandedListWidth = clampTableWidth(snapTableWidth(tableWidth))
  let listStyle: React.CSSProperties = {}
  if (isCondensed) {
    const width =
      listState === "collapsing" || isListCollapsed
        ? COLLAPSED_LIST_WIDTH
        : expandedListWidth
    let transition: string | undefined
    if (isListAnimating) {
      transition = `flex-basis ${TRANSITION_DURATION_MS}ms ${TRANSITION_EASING}`
    } else if (
      isSnapping ||
      (isDragging && expandedListWidth <= FULL_CONDENSED_WIDTH)
    ) {
      transition = `flex-basis ${SNAP_DURATION_MS}ms ${TRANSITION_EASING}`
    }
    listStyle = { flex: `0 0 ${width}px`, transition }
  }

  // The › next to ‹ widens the list one step at a time: first to the
  // default columns (★ / Trigger / Name / Status), then to every column.
  // Once every column shows (or the window has no room), it opens the full
  // table, like the table icon on top.
  const canShowAllColumns =
    clampTableWidth(FULL_CONDENSED_WIDTH) >= FULL_CONDENSED_WIDTH
  const hiddenColumns = hiddenCondensedColumns(expandedListWidth)
  const nextListWidth =
    expandedListWidth < CONDENSED_SNAP_WIDTHS[2]
      ? CONDENSED_SNAP_WIDTHS[2]
      : FULL_CONDENSED_WIDTH
  const nextHidden = hiddenCondensedColumns(nextListWidth)
  const columnsToShow = hiddenColumns.filter((c) => !nextHidden.includes(c))
  const canWidenList = hiddenColumns.length > 0 && canShowAllColumns
  // A quick double-click on › shouldn't widen the list *and* then close
  // the details, so the full-table step ignores clicks just after a widen.
  const lastWidenAt = useRef(0)
  const widenList = () => {
    lastWidenAt.current = Date.now()
    glideToWidth(nextListWidth)
  }
  const openFullTable = () => {
    if (Date.now() - lastWidenAt.current < FULL_TABLE_CLICK_GUARD_MS) {
      return
    }
    closeDetails()
  }

  return (
    <LastOpenedResourceProvider name={detailResource}>
      <OverviewTablePaneStyle ref={rootRef}>
        <TransitionStyle />
        <OverviewTableStickyNav>
          <HeaderBar
            view={props.view}
            currentPage={isOpen ? HeaderBarPage.Detail : HeaderBarPage.Grid}
            isSocketConnected={props.isSocketConnected}
            detailResource={detailResource}
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
              <BackToDetailsButton
                aria-label={`Back to details: ${detailLabel}`}
                title={`Back to details: ${detailLabel}`}
                onClick={() =>
                  navigate(pb.encpath`/r/${detailResource}/overview`)
                }
              >
                <ChevronLeftIcon role="presentation" />
              </BackToDetailsButton>
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
                  onClick={() => {
                    // Opens to the default columns; › again shows the rest.
                    setTableWidth(defaultTableWidth())
                    setListCollapsed(false)
                  }}
                >
                  <ChevronRightIcon role="presentation" />
                </ListToggleButton>
                <CollapsedErrorRail
                  view={props.view}
                  openName={isOpen ? name : ""}
                />
                <CollapsedEdge
                  aria-hidden="true"
                  title="Drag to open the resource list"
                  className={isDragging ? "isDragging" : ""}
                  onPointerDown={(e) => startListDrag(e, true)}
                />
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
                  {canWidenList ? (
                    <ListToggleButton
                      aria-label={`Show more columns: ${columnsToShow.join(
                        ", "
                      )}`}
                      title={`Show ${columnsToShow.join(", ")}`}
                      onClick={widenList}
                    >
                      <ChevronRightIcon role="presentation" />
                    </ListToggleButton>
                  ) : (
                    <ListToggleButton
                      aria-label="Open full table (closes details)"
                      title="Open full table (closes details)"
                      onClick={openFullTable}
                    >
                      <ChevronRightIcon role="presentation" />
                    </ListToggleButton>
                  )}
                </ListFilterRow>
                <ListDisplayOptions
                  resources={props.view.uiResources}
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
                // The layout follows the width the list is headed to, so
                // columns change once, right away. Only while collapsing or
                // expanding do they follow the width on screen.
                availableWidth={
                  isListAnimating
                    ? listWidth ?? expandedListWidth
                    : expandedListWidth
                }
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
              className={isDragging ? "isDragging" : ""}
              onPointerDown={(e) => startListDrag(e, false)}
              onKeyDown={onDividerKeyDown}
              onDoubleClick={() => glideToWidth(defaultTableWidth())}
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
    </LastOpenedResourceProvider>
  )
}
