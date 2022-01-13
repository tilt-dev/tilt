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
  const {
    options: { helpSearchBar },
    setOptions,
  } = useHelpSearchBarOptions()

  function setHelpSearchBar(newValue: string) {
    setOptions({ helpSearchBar: newValue })
  }

  let inputProps: Partial<StandardInputProps> = {
    startAdornment: (
      <InputAdornment position="start">
        <SearchSvg fill={Color.grayLightest} />
      </InputAdornment>
    ),
  }

  function handleKeyPress(e: any) {
    if ("Enter" == e.key) {
      searchDocs(helpSearchBar)
    } else {
      setHelpSearchBar(helpSearchBar+e.key)
    }
  }

  // only show the "x" to clear if there's any input to clear
  if (HelpSearchBar.length) {
    const onClearClick = () => setHelpSearchBar("")

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
      value={helpSearchBar ?? ""}
      placeholder="Search Tilt Docs..."
      InputProps={inputProps}
      variant="outlined"
      analyticsName="ui.web.HelpSearchBar"
      onKeyPress={handleKeyPress}
    />
  )
}
