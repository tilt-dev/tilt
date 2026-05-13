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

function getSystemTheme(): Theme {
  return window.matchMedia?.("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark"
}

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === "light" || stored === "dark") {
    return stored
  }
  return getSystemTheme()
}

function hasUserOverride(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== null
}

export function ThemeProvider(props: PropsWithChildren<{}>) {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)
  useEffect(() => {
    if (theme === "light") {
      document.body.setAttribute("data-theme", "light")
    } else {
      document.body.removeAttribute("data-theme")
    }
  }, [theme])

  // Follow system theme changes when user hasn't explicitly chosen
  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-color-scheme: light)")
    if (!mq) return

    function onChange() {
      if (!hasUserOverride()) {
        setTheme(getSystemTheme())
      }
    }

    mq.addEventListener("change", onChange)
    return () => mq.removeEventListener("change", onChange)
  }, [])

  const toggleTheme = useCallback(() => {
    setTheme((prev) => {
      const next = prev === "dark" ? "light" : "dark"
      // If toggling back to the system theme, clear the override
      if (next === getSystemTheme()) {
        localStorage.removeItem(STORAGE_KEY)
      } else {
        localStorage.setItem(STORAGE_KEY, next)
      }
      return next
    })
  }, [])

  return (
    <themeContext.Provider value={{ theme, toggleTheme }}>
      {props.children}
    </themeContext.Provider>
  )
}
