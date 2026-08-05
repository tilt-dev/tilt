import { RefObject, useEffect, useState } from "react"

// Tracks an element's rendered width. The viewport isn't a good proxy for the
// space the table actually has, since it shares the page with a collapsible
// sidebar. Returns 0 until the first measurement.
export function useElementWidth(ref: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0)

  useEffect(() => {
    const element = ref.current
    if (!element) {
      return
    }

    // jsdom has no ResizeObserver, so tests get a single static measurement.
    if (typeof ResizeObserver === "undefined") {
      setWidth(element.getBoundingClientRect().width)
      return
    }

    // Setting state here re-enters the observer, since collapsing a column
    // resizes the table. The browser reports that as a ResizeObserver loop
    // error, so hand the measurement off to the next frame.
    let frame = 0
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) {
        const nextWidth = entry.contentRect.width
        cancelAnimationFrame(frame)
        frame = requestAnimationFrame(() => setWidth(nextWidth))
      }
    })

    observer.observe(element)

    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
    }
  }, [ref])

  return width
}
