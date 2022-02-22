import {
  createContext,
  PropsWithChildren,
  useContext,
  useMemo,
  useState,
} from "react"

/**
 * The ResourceSelection state keeps track of what resources are selected for bulk actions to be performed on them.
 */

type ResourceSelectionContext = {
  selected: Set<string>
  isSelected: (resourceName: string) => boolean
  select: (...resourceNames: string[]) => void
  deselect: (...resourceNames: string[]) => void
  clearSelections: () => void
}

const ResourceSelectionContext = createContext<ResourceSelectionContext>({
  selected: new Set(),
  isSelected: (_resourceName: string) => {
    console.warn("Resource selections context is not set.")
    return false
  },
  select: (..._resourceNames: string[]) => {
    console.warn("Resource selections context is not set.")
  },
  deselect: (..._resourceNames: string[]) => {
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
  const selections = new Set(props.initialValuesForTesting) || new Set()
  const [selectedResources, setSelectedResources] = useState(selections)

  const contextValue: ResourceSelectionContext = useMemo(() => {
    function isSelected(resourceName: string) {
      return selectedResources.has(resourceName)
    }

    function select(...resourceNames: string[]) {
      const newSelections = new Set<string>(selectedResources)
      resourceNames.forEach((name) => newSelections.add(name))
      return setSelectedResources(newSelections)
    }

    function deselect(...resourceNames: string[]) {
      const newSelections = new Set<string>(selectedResources)
      resourceNames.forEach((name) => newSelections.delete(name))
      return setSelectedResources(newSelections)
    }

    function clearSelections() {
      setSelectedResources(new Set())
    }
    return {
      selected: selectedResources,
      isSelected,
      select,
      deselect,
      clearSelections,
    }
  }, [selectedResources, setSelectedResources])

  return (
    <ResourceSelectionContext.Provider value={contextValue}>
      {props.children}
    </ResourceSelectionContext.Provider>
  )
}
