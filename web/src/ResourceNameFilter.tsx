import { InputAdornment } from "@material-ui/core"
import { InputProps as StandardInputProps } from "@material-ui/core/Input/Input"
import React from "react"
import styled from "styled-components"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as SearchSvg } from "./assets/svg/search.svg"
import {
  InstrumentedButton,
  InstrumentedTextField,
} from "./instrumentedComponents"
import { useResourceListOptions } from "./ResourceListOptionsContext"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export function matchesResourceName(
  resourceName: string,
  filter: string
): boolean {
  filter = filter.trim()
  // this is functionally redundant but probably an important enough case to make its own thing
  if (filter === "") {
    return true
  }
  // a resource matches the query if the resource name contains all tokens in the query
  return filter
    .split(" ")
    .every((token) => resourceName.toLowerCase().includes(token.toLowerCase()))
}

export const ResourceNameFilterTextField = styled(InstrumentedTextField)`
  & .MuiOutlinedInput-root {
    border-radius: ${SizeUnit(0.5)};
    border: 1px solid ${Color.gray40};
    background-color: ${Color.gray30};

    & fieldset {
      border-color: 1px solid ${Color.gray40};
    }
    &:hover fieldset {
      border: 1px solid ${Color.gray40};
    }
    &.Mui-focused fieldset {
      border: 1px solid ${Color.gray40};
    }
    & .MuiOutlinedInput-input {
      padding: ${SizeUnit(0.2)};
    }
  }

  margin-top: ${SizeUnit(0.4)};
  margin-bottom: ${SizeUnit(0.4)};

  & .MuiInputBase-input {
    font-family: ${Font.monospace};
    color: ${Color.offWhite};
    font-size: ${FontSize.small};
  }
`

export const ClearResourceNameFilterButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
`

export function ResourceNameFilter(props: { className?: string }) {
  const {
    options: { resourceNameFilter },
    setOptions,
  } = useResourceListOptions()

  function setResourceNameFilter(newValue: string) {
    setOptions({ resourceNameFilter: newValue })
  }

  let inputProps: Partial<StandardInputProps> = {
    "aria-label": "Filter resources by name",
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={Color.grayLightest} />
      </InputAdornment>
    ),
  }

  // only show the "x" to clear if there's any input to clear
  if (resourceNameFilter.length) {
    const onClearClick = () => setResourceNameFilter("")

    inputProps.endAdornment = (
      <InputAdornment position="end">
        <ClearResourceNameFilterButton
          onClick={onClearClick}
          analyticsName="ui.web.clearResourceNameFilter"
          aria-label="Clear name filter"
        >
          <CloseSvg role="presentation" fill={Color.grayLightest} />
        </ClearResourceNameFilterButton>
      </InputAdornment>
    )
  }

  return (
    <ResourceNameFilterTextField
      className={props.className}
      value={resourceNameFilter ?? ""}
      onChange={(e) => setResourceNameFilter(e.target.value)}
      placeholder="Filter resources by name"
      InputProps={inputProps}
      variant="outlined"
      analyticsName="ui.web.resourceNameFilter"
    />
  )
}
