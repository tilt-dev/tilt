import React, { MouseEventHandler } from "react"
import styled from "styled-components"
import { Tags } from "./analytics"
import { InstrumentedButton } from "./instrumentedComponents"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"

const ShowMoreButtonRoot = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  color: ${Color.gray60};
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
  padding: 0 0.5em;
  transition: color ${AnimDuration.default} ease;

  &:hover,
  &:focus,
  &:active {
    color: ${Color.blue};
  }
`

const ShowMoreCount = styled.span`
  color: ${Color.gray70};
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
`

export function ShowMoreButton({
  itemCount,
  currentListSize,
  onClick,
  analyticsTags,
}: {
  itemCount: number
  currentListSize: number
  analyticsTags: Tags
  onClick: MouseEventHandler
}) {
  if (itemCount <= currentListSize) {
    return null
  }

  const remainingCount = itemCount - currentListSize

  return (
    <>
      <ShowMoreButtonRoot
        analyticsName="ui.web.showMore"
        analyticsTags={analyticsTags}
        onClick={onClick}
      >
        …Show more
      </ShowMoreButtonRoot>
      <ShowMoreCount aria-label={`${remainingCount} hidden resources`}>
        ({remainingCount})
      </ShowMoreCount>
    </>
  )
}
