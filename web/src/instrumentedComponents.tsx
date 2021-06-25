import { LegacyRef } from "react"
import { incr, Tags } from "./analytics"

export type InstrumentedButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  analyticsName: string
  analyticsTags?: Tags
  ref?: LegacyRef<HTMLButtonElement>
}

export function InstrumentedButton(props: InstrumentedButtonProps) {
  const { analyticsName, analyticsTags, onClick, ...buttonProps } = { ...props }
  const tags = { action: "click", ...(analyticsTags ?? {}) }
  const instrumentedOnClick = (
    e: React.MouseEvent<HTMLButtonElement, MouseEvent>
  ) => {
    incr(analyticsName, tags)
    if (onClick) {
      onClick(e)
    }
  }
  return <button onClick={instrumentedOnClick} {...buttonProps} />
}
