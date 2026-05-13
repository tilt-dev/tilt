import React, { PropsWithChildren, useCallback, useContext, useEffect, useState } from "react"

export type Theme = "dark" | "light"

export type ThemeContextType = {
  theme: Theme
  toggleTheme: () => void
}

const themeContext = React.createContext<ThemeContextType>({
  theme: "dark",
  toggleTheme: () => {},
})

export function useTheme(): ThemeContextType {
  return useContext(themeContext)
}

const STORAGE_KEY = "tilt-theme"

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === "light" || stored === "dark") {
    return stored
  }
  if (window.matchMedia?.("(prefers-color-scheme: light)").matches) {
    return "light"
  }
  return "dark"
}

export function ThemeProvider(props: PropsWithChildren<{}>) {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    if (theme === "light") {
      document.body.setAttribute("data-theme", "light")
    } else {
      document.body.removeAttribute("data-theme")
    }
    localStorage.setItem(STORAGE_KEY, theme)
  }, [theme])

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"))
  }, [])

  return (
    <themeContext.Provider value={{ theme, toggleTheme }}>
      {props.children}
    </themeContext.Provider>
  )
}
