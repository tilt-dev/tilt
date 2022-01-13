import React, { ChangeEvent, useState } from "react"
import { InputAdornment } from "@material-ui/core"
import { InputProps as StandardInputProps } from "@material-ui/core/Input/Input"
import styled from "styled-components"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as SearchSvg } from "./assets/svg/search.svg"
import {
  InstrumentedButton,
  InstrumentedTextField,
} from "./instrumentedComponents"
import { useHelpSearchBarOptions } from "./HelpSearchBarOptionsContext"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export function searchDocs(
  query: string
) {
  window.open("https://docs.tilt.dev/search?q=tilt")
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

export function HelpSearchBar(props: { className?: string }) {
  const [ searchValue, setSearchValue ] = useState("")

  let inputProps: Partial<StandardInputProps> = {
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={Color.grayLightest} />
      </InputAdornment>
    ),
  }

  function handleKeyPress(e: any) {
    if ("Enter" === e.key) {
      searchDocs(searchValue)
      setSearchValue("")
    }
  }

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const { value } = e.target
    console.log(value)
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
      className={props.className}
      value={searchValue}
      placeholder="Search Tilt Docs..."
      InputProps={inputProps}
      variant="outlined"
      analyticsName="ui.web.HelpSearchBar"
      onKeyPress={handleKeyPress}
      onChange={handleChange}
    />
  )
}
