// Types and parsing logic for log filters.

import { Location } from "history"
import { useHistory } from "react-router"

export const EMPTY_TERM = ""

export enum FilterLevel {
  all = "",

  // Only show warnings.
  warn = "warn",

  // Only show errors.
  error = "error",
}

export enum FilterSource {
  all = "",

  // Only show build logs.
  build = "build",

  // Only show runtime logs.
  runtime = "runtime",
}

// The term could have more metadata associated with it
interface FilterTerm {
  type: "string" | "regex"
  content: string | null
  union: "include" | "exclude"
}

export type FilterSet = {
  level: FilterLevel
  source: FilterSource
  term?: string // Optional for now; Is an empty string an acceptable empty state?
}

// Infers filter set from the history React hook.
export function useFilterSet(): FilterSet {
  return filterSetFromLocation(useHistory().location)
}

// The source of truth for log filters is the URL.
// For example,
// /r/(all)/overview?level=error&source=build
// will only show errors from the build, not from the pod.
export function filterSetFromLocation(l: Location): FilterSet {
  let params = new URLSearchParams(l.search)
  let filters = {
    level: FilterLevel.all,
    source: FilterSource.all,
    term: EMPTY_TERM,
  }
  switch (params.get("level")) {
    case FilterLevel.warn:
      filters.level = FilterLevel.warn
      break
    case FilterLevel.error:
      filters.level = FilterLevel.error
      break
  }

  switch (params.get("source")) {
    case FilterSource.build:
      filters.source = FilterSource.build
      break
    case FilterSource.runtime:
      filters.source = FilterSource.runtime
      break
  }

  const filterTerm = params.get("term")
  if (filterTerm) {
    // TODO: Apply some form of term standardization, like lowercasing and trimming
    filters.term = filterTerm.toLowerCase()
  } else {
    filters.term = EMPTY_TERM
  }

  return filters
}

export function filterSetsEqual(a: FilterSet, b: FilterSet): boolean {
  return a.source === b.source && a.level === b.level && a.term === b.term
}
