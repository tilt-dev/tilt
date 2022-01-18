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
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export function searchDocs(query: string) {
  const docsSearchUrl = new URL("https://docs.tilt.dev/search")
  docsSearchUrl.searchParams.set("q", query)
  docsSearchUrl.searchParams.set("utm_source", "tiltui")
  window.open(docsSearchUrl)
}

export const HelpSearchBarTextField = styled(InstrumentedTextField)`
  & .MuiOutlinedInput-root {
    border-radius: ${SizeUnit(0.5)};
    background-color: ${Color.white};

    & fieldset {
      border-color: 1px solid ${Color.grayLighter};
    }
    &:hover fieldset {
      border: 1px solid ${Color.grayLighter};
    }
    &.Mui-focused fieldset {
      border: 1px solid ${Color.grayLighter};
    }
    & .MuiOutlinedInput-input {
      padding: ${SizeUnit(0.2)};
    }
  }

  margin-top: ${SizeUnit(0.4)};
  margin-bottom: ${SizeUnit(0.4)};

  & .MuiInputBase-input {
    font-family: ${Font.monospace};
    color: ${Color.grayLighter};
    font-size: ${FontSize.small};
  }
`

export const ClearHelpSearchBarButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
`
export let searchValue: string
let setSearchValue: React.Dispatch<React.SetStateAction<string>>

export function HelpSearchBar(props: { className?: string }) {
  ;[searchValue, setSearchValue] = useState("")

  let inputProps: Partial<StandardInputProps> = {
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={Color.grayLightest} />
      </InputAdornment>
    ),
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
          analyticsName="ui.web.clearHelpSearchBar"
        >
          <CloseSvg fill={Color.grayLightest} />
        </ClearHelpSearchBarButton>
      </InputAdornment>
    )
  }

  return (
    <HelpSearchBarTextField
      aria-label="Search Tilt Docs"
      className={props.className}
      value={searchValue}
      placeholder="Search Tilt Docs..."
      InputProps={inputProps}
      variant="outlined"
      analyticsName="ui.web.helpSearchBar"
      onKeyPress={handleKeyPress}
      onChange={handleChange}
    />
  )
}
