import { createContext, PropsWithChildren, useContext, useState } from "react"

/**
 * The ResourceSelection state keeps track of what resources are selected for bulk actions to be performed on them.
 */

type ResourceSelectionContext = {
  selected: string[]
  isSelected: (resourceName: string) => boolean
  select: (resourceName: string) => void
  deselect: (resourceName: string) => void
  clearSelections: () => void
}

const ResourceSelectionContext = createContext<ResourceSelectionContext>({
  selected: [],
  isSelected: (_resourceName: string) => {
    console.warn("Resource selections context is not set.")
    return false
  },
  select: (_resourceName: string) => {
    console.warn("Resource selections context is not set.")
  },
  deselect: (_resourceName: string) => {
    console.warn("Resource selections context is not set.")
  },
  clearSelections: () => {
    console.warn("Resource selections context is not set.")
  },
})

ResourceSelectionContext.displayName = "ResourceSelectionContext"

export function useResourceSelection(): ResourceSelectionContext {
  return useContext(ResourceSelectionContext)
}

export function ResourceSelectionProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: string[] }>
) {
  const selections = props.initialValuesForTesting || []
  const [selectedResources, setSelectedResources] = useState(selections)

  function isSelected(resourceName: string) {
    return selectedResources.includes(resourceName)
  }

  function select(resourceName: string) {
    return setSelectedResources([...selectedResources, resourceName])
  }

  function deselect(resourceName: string) {
    return setSelectedResources(
      selectedResources.filter((r) => r !== resourceName)
    )
  }

  function clearSelections() {
    setSelectedResources([])
  }

  const contextValue: ResourceSelectionContext = {
    selected: selectedResources,
    isSelected,
    select,
    deselect,
    clearSelections,
  }

  return (
    <ResourceSelectionContext.Provider value={contextValue}>
      {props.children}
    </ResourceSelectionContext.Provider>
  )
}
