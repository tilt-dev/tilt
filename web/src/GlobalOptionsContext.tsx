import { createContext, PropsWithChildren, useContext } from "react"
import { usePersistentState } from "./LocalStorage"

export type GlobalOptions = {
  resourceNameFilter: string
}

type GlobalOptionsContext = {
  options: GlobalOptions
  setOptions: (options: Partial<GlobalOptions>) => void
}

export const DEFAULT_OPTIONS: GlobalOptions = {
  resourceNameFilter: "",
}

const globalOptionsContext = createContext<GlobalOptionsContext>({
  options: { ...DEFAULT_OPTIONS },
  setOptions: () => {
    console.warn("Global options context is not set.")
  },
})

export function useGlobalOptions(): GlobalOptionsContext {
  return useContext(globalOptionsContext)
}

export function GlobalOptionsContextProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: GlobalOptions }>
) {
  const defaultPersistentValue = props.initialValuesForTesting ?? {
    ...DEFAULT_OPTIONS,
  }
  const [options, setGlobalOptions] = usePersistentState<GlobalOptions>(
    "global-options",
    defaultPersistentValue
  )

  function setOptions(options: Partial<GlobalOptions>) {
    setGlobalOptions((previousOptions) => ({ ...previousOptions, ...options }))
  }

  const defaultValue: GlobalOptionsContext = {
    options,
    setOptions,
  }

  return (
    <globalOptionsContext.Provider value={defaultValue}>
      {props.children}
    </globalOptionsContext.Provider>
  )
}
