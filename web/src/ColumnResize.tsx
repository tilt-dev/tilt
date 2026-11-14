import React, { PropsWithChildren, useContext, useMemo, useRef } from "react"
import styled from "styled-components"
import { usePersistentState } from "./BrowserStorage"
import { Color } from "./style-helpers"

// Resizable table columns.
//
// Each column has a default and a minimum width. One column is "flexible":
// it has no stored width and takes whatever space is left. Dragging the edge
// between two columns moves width from one to the other, so the table never
// grows wider than its container.

export type ColumnSizing = {
  defaults: { [id: string]: number }
  minimums: { [id: string]: number }
  flexibleColumn: string
}

type ColumnWidthsContext = {
  sizing: ColumnSizing
  widths: { [id: string]: number }
  setWidths: (widths: { [id: string]: number }) => void
}

const columnWidthsContext = React.createContext<ColumnWidthsContext | null>(
  null
)

// Returns null when columns aren't resizable.
export function useColumnWidths(): ColumnWidthsContext | null {
  return useContext(columnWidthsContext)
}

// Without `sizing`, columns aren't resizable. The provider still renders,
// so turning resizing on and off doesn't remount (and reset) the table.
export function ColumnWidthsProvider(
  props: PropsWithChildren<{ name: string; sizing?: ColumnSizing }>
) {
  const [stored, setStored] = usePersistentState<{ [id: string]: number }>(
    props.name,
    {}
  )
  const { sizing } = props
  const value = useMemo(
    () =>
      sizing
        ? {
            sizing,
            widths: { ...sizing.defaults, ...stored },
            setWidths: setStored,
          }
        : null,
    [sizing, stored, setStored]
  )
  return (
    <columnWidthsContext.Provider value={value}>
      {props.children}
    </columnWidthsContext.Provider>
  )
}

const ResizeHandle = styled.div`
  position: absolute;
  top: 0;
  right: -5px;
  width: 10px;
  height: 100%;
  cursor: col-resize;
  z-index: 1;
  touch-action: none;

  &::after {
    content: "";
    position: absolute;
    top: 25%;
    bottom: 25%;
    left: 4px;
    width: 2px;
    background-color: ${Color.gray40};
  }

  &:hover::after,
  &.isDragging::after {
    top: 0;
    bottom: 0;
    background-color: ${Color.blue};
  }
`

// A drag handle on the right edge of a header cell, between `columnId`
// and `nextColumnId`. The header cell must be `position: relative`.
// Double-click resets both columns to their defaults.
export function ColumnResizeHandle(props: {
  columnId: string
  nextColumnId: string
}) {
  const ctx = useColumnWidths()
  const ref = useRef<HTMLDivElement>(null)
  if (!ctx) {
    return null
  }
  const { sizing, widths, setWidths } = ctx
  const { columnId, nextColumnId } = props

  const onPointerDown = (e: React.PointerEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const handle = ref.current
    const leftCell = handle?.parentElement
    const rightCell = leftCell?.nextElementSibling
    if (!handle || !leftCell || !rightCell) {
      return
    }

    // Measure what's actually on screen, since the flexible column's width
    // depends on the container.
    const startX = e.clientX
    const startLeft = leftCell.getBoundingClientRect().width
    const startRight = rightCell.getBoundingClientRect().width
    const minLeft = sizing.minimums[columnId] ?? 0
    const minRight = sizing.minimums[nextColumnId] ?? 0

    handle.setPointerCapture(e.pointerId)
    handle.classList.add("isDragging")

    const onMove = (e: PointerEvent) => {
      let dx = e.clientX - startX
      dx = Math.max(dx, minLeft - startLeft)
      dx = Math.min(dx, startRight - minRight)
      const next = { ...widths }
      if (columnId !== sizing.flexibleColumn) {
        next[columnId] = Math.round(startLeft + dx)
      }
      if (nextColumnId !== sizing.flexibleColumn) {
        next[nextColumnId] = Math.round(startRight - dx)
      }
      setWidths(next)
    }
    const onUp = () => {
      handle.classList.remove("isDragging")
      handle.removeEventListener("pointermove", onMove)
      handle.removeEventListener("pointerup", onUp)
      handle.removeEventListener("pointercancel", onUp)
    }
    handle.addEventListener("pointermove", onMove)
    handle.addEventListener("pointerup", onUp)
    handle.addEventListener("pointercancel", onUp)
  }

  const onDoubleClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    const next = { ...widths }
    delete next[columnId]
    delete next[nextColumnId]
    setWidths(next)
  }

  return (
    <ResizeHandle
      ref={ref}
      role="separator"
      aria-orientation="vertical"
      aria-label={`Resize column`}
      title="Drag to resize · double-click to reset"
      onPointerDown={onPointerDown}
      onDoubleClick={onDoubleClick}
      // Don't let a drag end as a click that sorts the column
      onClick={(e) => e.stopPropagation()}
    />
  )
}
