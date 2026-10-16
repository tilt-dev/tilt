import { useCallback } from "react"
import { useLocation, useNavigate } from "react-router-dom"

// The source of truth for the selected log line is the URL, so that a
// selection can be shared as a permalink. For example,
// /r/(all)/overview?line=1234
// opens the log details drawer for the line with storedLineIndex 1234.
export const SELECTED_LINE_PARAM = "line"

export function selectedLineFromSearch(search: string): number | null {
  let value = new URLSearchParams(search).get(SELECTED_LINE_PARAM)
  if (value === null || value === "") {
    return null
  }
  let index = Number(value)
  return Number.isInteger(index) && index >= 0 ? index : null
}

export function searchWithSelectedLine(
  search: string,
  index: number | null
): string {
  let params = new URLSearchParams(search)
  if (index === null) {
    params.delete(SELECTED_LINE_PARAM)
  } else {
    params.set(SELECTED_LINE_PARAM, String(index))
  }
  let str = params.toString()
  return str ? `?${str}` : ""
}

export function useSelectedLogLine(): [
  number | null,
  (index: number | null) => void
] {
  let location = useLocation()
  let navigate = useNavigate()
  let selected = selectedLineFromSearch(location.search)

  let setSelected = useCallback(
    (index: number | null) => {
      if (index === selectedLineFromSearch(location.search)) {
        return
      }
      navigate(
        {
          pathname: location.pathname,
          search: searchWithSelectedLine(location.search, index),
        },
        { replace: true }
      )
    },
    [location.pathname, location.search, navigate]
  )

  return [selected, setSelected]
}
