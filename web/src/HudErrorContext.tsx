import React, { useContext } from "react"

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
