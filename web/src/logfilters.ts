// Types and parsing logic for log filters.

import { Location } from "history"
import { useLocation } from "react-router"
import RegexEscape from "regex-escape"

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

export enum TermState {
  Empty = "empty",
  Parsed = "parsed",
  Error = "error",
}

type EmptyTerm = { state: TermState.Empty }

type ParsedTerm = { state: TermState.Parsed; regexp: RegExp }

type ErrorTerm = { state: TermState.Error; error: string }

type FilterTerm = {
  input: string // Unmodified string input
} & (EmptyTerm | ParsedTerm | ErrorTerm)

export type FilterSet = {
  level: FilterLevel
  source: FilterSource
  term: FilterTerm
}

export const EMPTY_TERM = ""
export const EMPTY_FILTER_TERM: FilterTerm = {
  input: EMPTY_TERM,
  state: TermState.Empty,
}

export function parseTermInput(input: string): RegExp {
  // Escape all characters that have special meaning in RegExp,
  // so term can be treated as a string literal
  const escapedInput = RegexEscape(input)

  // Filter terms are case-insensitive and can match multiple instances
  return new RegExp(escapedInput, "gi")
}

export function createFilterTermState(input: string): FilterTerm {
  if (!input) {
    return EMPTY_FILTER_TERM
  }

  try {
    return {
      input,
      regexp: parseTermInput(input),
      state: TermState.Parsed,
    }
  } catch (error) {
    return {
      input,
      state: TermState.Error,
      error: error?.message,
    }
  }
}

// Infers filter set from the history React hook.
export function useFilterSet(): FilterSet {
  return filterSetFromLocation(useLocation())
}

// The source of truth for log filters is the URL.
// For example,
// /r/(all)/overview?level=error&source=build&term=docker
// will only show errors from the build, not from the pod,
// and that include the string `docker`.
export function filterSetFromLocation(l: Location): FilterSet {
  let params = new URLSearchParams(l.search)
  let filters: FilterSet = {
    level: FilterLevel.all,
    source: FilterSource.all,
    term: EMPTY_FILTER_TERM,
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

  const input = params.get("term")
  if (input) {
    filters.term = createFilterTermState(input)
  }

  return filters
}

export function filterSetsEqual(a: FilterSet, b: FilterSet): boolean {
  const sourceEqual = a.source === b.source
  const levelEqual = a.level === b.level
  // Filter terms are case-insensitive, so we can ignore casing when comparing terms
  const termEqual = a.term.input.toLowerCase() === b.term.input.toLowerCase()
  return sourceEqual && levelEqual && termEqual
}
