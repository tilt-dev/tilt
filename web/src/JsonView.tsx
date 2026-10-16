import React from "react"
import styled from "styled-components"
import { JSONValue } from "./logparse"
import { Color, Font } from "./style-helpers"

// Pretty-prints a JSON value with syntax highlighting.

const JsonPre = styled.pre`
  margin: 0;
  font-family: ${Font.monospace};
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: ${Color.grayLightest};

  .json-key {
    color: ${Color.blueLight};
  }
  .json-string {
    color: ${Color.greenLight};
  }
  .json-number {
    color: ${Color.yellowLight};
  }
  .json-boolean {
    color: ${Color.purple};
  }
  .json-null {
    color: ${Color.gray60};
    font-style: italic;
  }
`

const indentUnit = "  "

function renderValue(value: JSONValue, depth: number): React.ReactNode {
  if (value === null) {
    return <span className="json-null">null</span>
  }

  switch (typeof value) {
    case "string":
      return <span className="json-string">{JSON.stringify(value)}</span>
    case "number":
      return <span className="json-number">{String(value)}</span>
    case "boolean":
      return <span className="json-boolean">{String(value)}</span>
  }

  let indent = indentUnit.repeat(depth)
  let childIndent = indentUnit.repeat(depth + 1)

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return "[]"
    }
    return (
      <>
        {"[\n"}
        {value.map((item, i) => (
          <React.Fragment key={i}>
            {childIndent}
            {renderValue(item, depth + 1)}
            {i < value.length - 1 ? ",\n" : "\n"}
          </React.Fragment>
        ))}
        {indent}]
      </>
    )
  }

  let keys = Object.keys(value)
  if (keys.length === 0) {
    return "{}"
  }
  return (
    <>
      {"{\n"}
      {keys.map((key, i) => (
        <React.Fragment key={key}>
          {childIndent}
          <span className="json-key">{JSON.stringify(key)}</span>
          {": "}
          {renderValue(value[key], depth + 1)}
          {i < keys.length - 1 ? ",\n" : "\n"}
        </React.Fragment>
      ))}
      {indent}
      {"}"}
    </>
  )
}

export default function JsonView(props: { value: JSONValue }) {
  return (
    <JsonPre aria-label="Formatted JSON">{renderValue(props.value, 0)}</JsonPre>
  )
}
