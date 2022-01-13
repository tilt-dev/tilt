import { createContext, PropsWithChildren, useContext } from "react"
import { useSessionState } from "./BrowserStorage"

/**
 * The HelpSearchBarOptions state keeps track of filters and sorting
 * that are applied to resource lists and used across views (with exceptions).
 *
 * As the persistent options for resource listing change, this context may
 * need to be refactored or reconsidered.
 */

// Note: this state was renamed from `SidebarOptions` to `ResourceListOptions`,
// but the local storage key was kept the same
export const HELP_SEARCH_BAR_OPTIONS_KEY = "help_searchbar_options"

export type HelpSearchBarOptions = {
  helpSearchBar: string
}

type HelpSearchBarOptionsContext = {
  options: HelpSearchBarOptions
  setOptions: (options: Partial<HelpSearchBarOptions>) => void
}

export const DEFAULT_OPTIONS: HelpSearchBarOptions = {
  helpSearchBar: "",
}

const HelpSearchBarOptionsContext = createContext<HelpSearchBarOptionsContext>({
  options: { ...DEFAULT_OPTIONS },
  setOptions: () => {
    console.warn("Help Searchbar options context is not set.")
  },
})

// Note: non-nullable fields added to `HelpSearchBarOptions` (formerly `SidebarOptions`)
// after its initial release need to have default values filled in here
function maybeUpgradeSavedOptions(savedOptions: HelpSearchBarOptions) {
  return {
    ...savedOptions,
    resourceNameFilter: savedOptions.helpSearchBar ?? "",
  }
}

export function useHelpSearchBarOptions(): HelpSearchBarOptionsContext {
  return useContext(HelpSearchBarOptionsContext)
}

export function HelpSearchBarOptionsProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: HelpSearchBarOptions }>
) {
  const defaultPersistentValue = props.initialValuesForTesting ?? {
    ...DEFAULT_OPTIONS,
  }
  const [options, setHelpSearchBarOptions] =
    useSessionState<HelpSearchBarOptions>(
      HELP_SEARCH_BAR_OPTIONS_KEY,
      defaultPersistentValue,
      maybeUpgradeSavedOptions
    )

  function setOptions(options: Partial<HelpSearchBarOptions>) {
    setHelpSearchBarOptions((previousOptions) => ({
      ...previousOptions,
      ...options,
    }))
  }

  const defaultValue: HelpSearchBarOptionsContext = {
    options,
    setOptions,
  }

  return (
    <HelpSearchBarOptionsContext.Provider value={defaultValue}>
      {props.children}
    </HelpSearchBarOptionsContext.Provider>
  )
}
