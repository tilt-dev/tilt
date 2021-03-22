import React, { useEffect } from "react"
import { useStorageState } from "react-storage-hooks"
import styled from "styled-components"
import { incr } from "./analytics"
import ClearLogs from "./ClearLogs"
import { Color, FontSize, mixinResetButtonStyle } from "./style-helpers"

export const LogFontSizeScaleLocalStorageKey = "tilt.global.log-font-scale"
export const LogFontSizeScaleCSSProperty = "--log-font-scale"
export const LogFontSizeScaleMinimumPercentage = 10

const LogActionsGroup = styled.div`
  margin-left: auto;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
`

const FontSizeControls = styled.div`
  color: ${Color.gray6};
  vertical-align: middle;
  display: flex;
  flex-wrap: nowrap;
`

const FontSizeControlsDivider = styled.div`
  font-size: ${FontSize.default};
  &:after {
    content: "|";
  }
`

const FontSizeButton = styled.button`
  ${mixinResetButtonStyle}
  color: ${Color.gray6};

  padding: 0 4px;

  &:after {
    content: "A";
  }
`

const FontSizeDecreaseButton = styled(FontSizeButton)`
  font-size: ${FontSize.smallest};
`
FontSizeDecreaseButton.displayName = "LogFontSizeDecreaseButton"

const FontSizeIncreaseButton = styled(FontSizeButton)`
  font-size: ${FontSize.default};
`
FontSizeIncreaseButton.displayName = "LogFontSizeIncreaseButton"

export const LogsFontSize: React.FC = () => {
  // this uses `useStorageState` directly vs `usePersistentState` wrapper because it's a global setting
  // (i.e. log zoom applies across all Tiltfiles)
  const [logFontScale, setLogFontSize] = useStorageState<string>(
    localStorage,
    LogFontSizeScaleLocalStorageKey,
    () =>
      document.documentElement.style.getPropertyValue(
        LogFontSizeScaleCSSProperty
      )
  )
  useEffect(() => {
    if (!logFontScale?.endsWith("%")) {
      return
    }
    document.documentElement.style.setProperty(
      LogFontSizeScaleCSSProperty,
      logFontScale
    )
  }, [logFontScale])

  const adjustLogFontScale = (step: number) => {
    const val = Math.max(
      parseFloat(logFontScale) + step,
      LogFontSizeScaleMinimumPercentage
    )
    if (isNaN(val)) {
      return
    }
    setLogFontSize(`${val}%`)
    incr("ui.web.zoomLogs", { action: "click", dir: step < 0 ? "out" : "in" })
  }

  const zoomStep = 5
  return (
    <FontSizeControls>
      <FontSizeDecreaseButton
        aria-label={"Decrease log font size"}
        onClick={() => adjustLogFontScale(-zoomStep)}
      />
      <FontSizeControlsDivider aria-hidden={true} />
      <FontSizeIncreaseButton
        aria-label={"Increase log font size"}
        onClick={() => adjustLogFontScale(zoomStep)}
      />
    </FontSizeControls>
  )
}

export interface LogActionsProps {
  resourceName: string
  isSnapshot: boolean
}

const LogActions: React.FC<LogActionsProps> = ({
  resourceName,
  isSnapshot,
}) => {
  return (
    <LogActionsGroup>
      <LogsFontSize />
      {isSnapshot || <ClearLogs resourceName={resourceName} />}
    </LogActionsGroup>
  )
}

export default LogActions
