// Helpers for detecting and parsing structured (JSON / logfmt) log lines,
// so that the log pane can summarize them and the details drawer can
// pretty-print them.
//
// Inspired by Dozzle's logparse package.

import { stripAnsiCodes } from "./logs"

export type JSONValue =
  | string
  | number
  | boolean
  | null
  | JSONValue[]
  | { [key: string]: JSONValue }

export type JSONObject = { [key: string]: JSONValue }

export enum StructuredKind {
  Text = "text",
  JSON = "json",
  Logfmt = "logfmt",
}

export type ParsedLogLine =
  | { kind: StructuredKind.Text }
  | {
      kind: StructuredKind.JSON | StructuredKind.Logfmt
      // Any text before a trailing JSON object, e.g. "INFO\tserver\t{...}"
      prefix: string
      data: JSONObject
    }

const TEXT: ParsedLogLine = { kind: StructuredKind.Text }

// Text before a trailing JSON object longer than this is probably prose that
// happens to end in JSON.
const maxJSONPrefixLen = 256

// Don't bother parsing enormous lines.
const maxParseLen = 64 * 1024

// Keys that hold the human-readable message, in priority order.
export const messageKeys = ["msg", "message", "@m", "@mt", "event"]

// Keys that hold the log level, in priority order.
export const levelKeys = ["level", "lvl", "severity", "@l", "log.level"]

function isPlainObject(v: unknown): v is JSONObject {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

function tryParseObject(text: string): JSONObject | null {
  try {
    let v = JSON.parse(text)
    return isPlainObject(v) ? v : null
  } catch (e) {
    return null
  }
}

// Parses a logfmt line (`key=value key2="quoted value"`).
// Returns null if the line isn't valid logfmt or has fewer than 2 pairs
// (to avoid treating prose like "retrying in 5s, attempt=3" as structured).
export function parseLogfmt(line: string): JSONObject | null {
  let result: JSONObject = {}
  let count = 0
  let i = 0
  let n = line.length

  while (i < n) {
    // skip whitespace between pairs
    while (i < n && line[i] === " ") i++
    if (i >= n) break

    let keyStart = i
    while (i < n && line[i] !== "=" && line[i] !== " " && line[i] !== '"') i++
    if (i >= n || line[i] !== "=" || i === keyStart) {
      return null
    }
    let key = line.substring(keyStart, i)
    i++ // skip '='

    let value: string
    if (line[i] === '"') {
      i++
      let buf = ""
      let closed = false
      while (i < n) {
        let c = line[i]
        if (c === "\\" && i + 1 < n) {
          let next = line[i + 1]
          buf += next === "n" ? "\n" : next === "t" ? "\t" : next
          i += 2
          continue
        }
        if (c === '"') {
          closed = true
          i++
          break
        }
        buf += c
        i++
      }
      if (!closed || (i < n && line[i] !== " ")) {
        return null
      }
      value = buf
    } else {
      let valStart = i
      while (i < n && line[i] !== " ") i++
      value = line.substring(valStart, i)
    }

    result[key] = value
    count++
  }

  return count >= 2 ? result : null
}

// Detects whether a log line is JSON, prefixed JSON, or logfmt, and parses it.
export function parseStructured(rawText: string): ParsedLogLine {
  if (!rawText || rawText.length > maxParseLen) {
    return TEXT
  }

  let text = stripAnsiCodes(rawText).trim()
  if (text.length < 3) {
    return TEXT
  }

  if (text[text.length - 1] === "}") {
    // Whole line is a JSON object.
    if (text[0] === "{") {
      let data = tryParseObject(text)
      if (data) {
        return { kind: StructuredKind.JSON, prefix: "", data }
      }
    }

    // Some text, then a JSON object, e.g. zap's console encoder:
    //   INFO	finished call	{"grpc.code": "OK"}
    let start = text.indexOf("{")
    if (
      start > 0 &&
      start <= maxJSONPrefixLen &&
      (text[start - 1] === " " || text[start - 1] === "\t")
    ) {
      let data = tryParseObject(text.substring(start))
      if (data && Object.keys(data).length > 0) {
        let prefix = text.substring(0, start).trim()
        return { kind: StructuredKind.JSON, prefix, data }
      }
    }
  }

  if (text.indexOf("=") !== -1) {
    let data = parseLogfmt(text)
    if (data) {
      return { kind: StructuredKind.Logfmt, prefix: "", data }
    }
  }

  return TEXT
}

// Flattens nested objects into dotted keys. Arrays are kept as leaf values.
export function flattenObject(
  obj: JSONObject,
  prefix: string = ""
): [string, JSONValue][] {
  let result: [string, JSONValue][] = []
  for (let key of Object.keys(obj)) {
    let value = obj[key]
    let fullKey = prefix ? `${prefix}.${key}` : key
    if (isPlainObject(value) && Object.keys(value).length > 0) {
      result.push(...flattenObject(value, fullKey))
    } else {
      result.push([fullKey, value])
    }
  }
  return result
}

// Finds the first key from `keys` that's present with a primitive value.
export function findKey(obj: JSONObject, keys: string[]): string | null {
  for (let key of keys) {
    let v = obj[key]
    if (v !== undefined && v !== null && typeof v !== "object") {
      return key
    }
  }
  return null
}

// Formats a value for display in a compact one-line summary.
export function formatValueCompact(v: JSONValue): string {
  if (typeof v === "string") {
    return v
  }
  return JSON.stringify(v)
}
