import React, { useContext } from "react"

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
  return (
    <hudErrorContext.Provider value={{ setError: props.setError }}>
      {props.children}
    </hudErrorContext.Provider>
  )
}
