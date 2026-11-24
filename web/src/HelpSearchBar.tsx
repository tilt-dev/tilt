import { InputAdornment } from "@material-ui/core"
import { InputProps as StandardInputProps } from "@material-ui/core/Input/Input"
import React, { ChangeEvent, KeyboardEvent, useState } from "react"
import styled from "styled-components"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as SearchSvg } from "./assets/svg/search.svg"
import {
  InstrumentedButton,
  InstrumentedTextField,
} from "./instrumentedComponents"
import {
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export function searchDocs(query: string) {
  const docsSearchUrl = new URL("https://docs.tilt.dev/search.html")
  docsSearchUrl.searchParams.set("q", query)
  docsSearchUrl.searchParams.set("utm_source", "tiltui")
  window.open(docsSearchUrl)
}

// The search bar sits on a white menu in both themes,
// so its colors stay fixed instead of following the theme.
const fieldBackground = "#ffffff"
const fieldForeground = "#2D4D55"
const iconFill = "#93a1a1"

export const HelpSearchBarTextField = styled(InstrumentedTextField)`
  & .MuiOutlinedInput-root {
    border-radius: ${SizeUnit(0.5)};
    background-color: ${fieldBackground};

    & fieldset {
      border-color: 1px solid ${fieldForeground};
    }
    &:hover fieldset {
      border: 1px solid ${fieldForeground};
    }
    &.Mui-focused fieldset {
      border: 1px solid ${fieldForeground};
    }
    & .MuiOutlinedInput-input {
      padding: ${SizeUnit(0.2)};
    }
  }

  margin-top: ${SizeUnit(0.4)};
  margin-bottom: ${SizeUnit(0.4)};

  & .MuiInputBase-input {
    font-family: ${Font.monospace};
    color: ${fieldForeground};
    font-size: ${FontSize.small};
  }
`

export const ClearHelpSearchBarButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
`

export function HelpSearchBar(props: { className?: string }) {
  const [searchValue, setSearchValue] = useState("")

  let inputProps: Partial<StandardInputProps> = {
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={iconFill} />
      </InputAdornment>
    ),
    "aria-label": "Search Tilt Docs",
  }

  function handleKeyPress(e: KeyboardEvent) {
    if ("Enter" === e.key) {
      searchDocs(searchValue)
      setSearchValue("")
    }
  }

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const { value } = e.target
    setSearchValue(value)
  }

  // only show the "x" to clear if there's any input to clear
  if (searchValue.length) {
    const onClearClick = () => setSearchValue("")

    inputProps.endAdornment = (
      <InputAdornment position="end">
        <ClearHelpSearchBarButton
          onClick={onClearClick}
          aria-label="Clear search term"
        >
          <CloseSvg role="presentation" fill={iconFill} />
        </ClearHelpSearchBarButton>
      </InputAdornment>
    )
  }

  return (
    <HelpSearchBarTextField
      className={props.className}
      value={searchValue}
      placeholder="Search Tilt Docs..."
      InputProps={inputProps}
      variant="outlined"
      onKeyPress={handleKeyPress}
      onChange={handleChange}
    />
  )
}
