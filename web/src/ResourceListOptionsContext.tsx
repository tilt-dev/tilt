import { createContext, PropsWithChildren, useContext } from "react"
import { useSessionState } from "./BrowserStorage"

/**
 * The ResourceListOptions state keeps track of filters and sorting
 * that are applied to resource lists and used across views (with exceptions).
 *
 * As the persistent options for resource listing change, this context may
 * need to be refactored or reconsidered.
 */

// Note: this state was renamed from `SidebarOptions` to `ResourceListOptions`,
// but the local storage key was kept the same
export const RESOURCE_LIST_OPTIONS_KEY = "sidebar_options"

export type ResourceListOptions = {
  alertsOnTop: boolean // Note: this is only used/implemented in OverviewSidebar
  resourceNameFilter: string
  showDisabledResources: boolean
}

type ResourceListOptionsContext = {
  options: ResourceListOptions
  setOptions: (options: Partial<ResourceListOptions>) => void
}

export const DEFAULT_OPTIONS: ResourceListOptions = {
  alertsOnTop: false,
  resourceNameFilter: "",
  showDisabledResources: false,
}

const ResourceListOptionsContext = createContext<ResourceListOptionsContext>({
  options: { ...DEFAULT_OPTIONS },
  setOptions: () => {
    console.warn("Resource list options context is not set.")
  },
})

// Note: non-nullable fields added to `ResourceListOptions` (formerly `SidebarOptions`)
// after its initial release need to have default values filled in here
function maybeUpgradeSavedOptions(savedOptions: ResourceListOptions) {
  return {
    ...savedOptions,
    resourceNameFilter: savedOptions.resourceNameFilter ?? "",
    showDisabledResources: savedOptions.showDisabledResources ?? false,
  }
}

export function useResourceListOptions(): ResourceListOptionsContext {
  return useContext(ResourceListOptionsContext)
}

export function ResourceListOptionsProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: ResourceListOptions }>
) {
  const defaultPersistentValue = props.initialValuesForTesting ?? {
    ...DEFAULT_OPTIONS,
  }
  const [options, setResourceListOptions] =
    useSessionState<ResourceListOptions>(
      RESOURCE_LIST_OPTIONS_KEY,
      defaultPersistentValue,
      maybeUpgradeSavedOptions
    )

  function setOptions(options: Partial<ResourceListOptions>) {
    setResourceListOptions((previousOptions) => ({
      ...previousOptions,
      ...options,
    }))
  }

  const defaultValue: ResourceListOptionsContext = {
    options,
    setOptions,
  }

  return (
    <ResourceListOptionsContext.Provider value={defaultValue}>
      {props.children}
    </ResourceListOptionsContext.Provider>
  )
}
