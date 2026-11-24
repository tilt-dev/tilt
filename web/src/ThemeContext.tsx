import React, {
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react"

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

// The user's stored preference. "system" follows the OS color scheme.
// Defaults to "dark" when nothing is stored.
type ThemeSetting = Theme | "system"

const systemLightQuery = "(prefers-color-scheme: light)"

function getSystemTheme(): Theme {
  return window.matchMedia?.(systemLightQuery).matches ? "light" : "dark"
}

function getStoredSetting(): ThemeSetting {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === "light" || stored === "dark" || stored === "system") {
    return stored
  }
  return "dark"
}

export function ThemeProvider(props: PropsWithChildren<{}>) {
  const [setting, setSetting] = useState<ThemeSetting>(getStoredSetting)
  const [systemTheme, setSystemTheme] = useState<Theme>(getSystemTheme)
  const theme = setting === "system" ? systemTheme : setting

  useEffect(() => {
    if (theme === "light") {
      document.body.setAttribute("data-theme", "light")
    } else {
      document.body.removeAttribute("data-theme")
    }
  }, [theme])

  useEffect(() => {
    const mq = window.matchMedia?.(systemLightQuery)
    if (!mq) return

    function onChange() {
      setSystemTheme(getSystemTheme())
    }

    mq.addEventListener("change", onChange)
    return () => mq.removeEventListener("change", onChange)
  }, [])

  // Flip the visible theme. If the result matches the system theme,
  // store "system" so the UI follows future system changes.
  const toggleTheme = useCallback(() => {
    const next: Theme = theme === "dark" ? "light" : "dark"
    const nextSetting: ThemeSetting = next === systemTheme ? "system" : next
    localStorage.setItem(STORAGE_KEY, nextSetting)
    setSetting(nextSetting)
  }, [theme, systemTheme])

  return (
    <themeContext.Provider value={{ theme, toggleTheme }}>
      {props.children}
    </themeContext.Provider>
  )
}
