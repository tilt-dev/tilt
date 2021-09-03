import { SnackbarContent, SnackbarProvider } from "notistack"
import React, { forwardRef, PropsWithChildren } from "react"
import styled from "styled-components"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

const SnackbarContentRoot = styled(SnackbarContent)`
  background-color: ${Color.grayLightest};
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
  font-weight: 400;
  color: ${Color.grayDark};
  padding: ${SizeUnit(0.25)};
  border: 1px solid ${Color.grayLight};
  border-radius: ${SizeUnit(0.125)};
`

const SnackMessage = forwardRef<
  HTMLDivElement,
  { id: string | number; message: string | React.ReactNode }
>((props, ref) => {
  return <SnackbarContentRoot ref={ref}>{props.message}</SnackbarContentRoot>
})

export function TiltSnackbarProvider(
  props: PropsWithChildren<{ className?: string }>
) {
  return (
    <SnackbarProvider
      maxSnack={3}
      anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      autoHideDuration={6000}
      content={(key, message) => <SnackMessage id={key} message={message} />}
    >
      {props.children}
    </SnackbarProvider>
  )
}
