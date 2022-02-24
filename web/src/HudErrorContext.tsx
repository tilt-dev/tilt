import React, { useContext, useMemo } from "react"

// allows consumers to set an error message to show up in the HUD
type HudErrorContext = {
  setError: (error: string) => void
}

const hudErrorContext = React.createContext<HudErrorContext>({
  setError: () => {},
})

export function useHudErrorContext() {
  return useContext(hudErrorContext)
}

export function HudErrorContextProvider(
  props: React.PropsWithChildren<HudErrorContext>
) {
  let value = useMemo(() => {
    return { setError: props.setError }
  }, [props.setError])
  return (
    <hudErrorContext.Provider value={value}>
      {props.children}
    </hudErrorContext.Provider>
  )
}
