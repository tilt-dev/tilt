import { createContext, PropsWithChildren, useContext, useMemo } from "react"
import { usePersistentState } from "./BrowserStorage"

export type GroupState = { expanded: boolean }

export type GroupsState = {
  [key: string]: GroupState
}

type ResourceGroupsContext = {
  groups: GroupsState
  getGroup: (groupLabel: string) => GroupState
  toggleGroupExpanded: (groupLabel: string) => void
  expandAll: () => void
  collapseAll: (groups: string[]) => void
}

export const DEFAULT_EXPANDED_STATE = true
export const DEFAULT_GROUP_STATE: GroupState = {
  expanded: DEFAULT_EXPANDED_STATE,
}

const resourceGroupsContext = createContext<ResourceGroupsContext>({
  groups: {},
  toggleGroupExpanded: () => {
    console.warn("Resource group context is not set.")
  },
  getGroup: () => {
    console.warn("Resource group context is not set.")
    return { ...DEFAULT_GROUP_STATE }
  },
  expandAll: () => void 0,
  collapseAll: (groups: string[]) => void 0,
})

export function useResourceGroups(): ResourceGroupsContext {
  return useContext(resourceGroupsContext)
}

export function ResourceGroupsContextProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: GroupsState }>
) {
  const defaultPersistentValue = props.initialValuesForTesting ?? {}
  const [groups, setGroups] = usePersistentState<GroupsState>(
    "resource-groups",
    defaultPersistentValue
  )

  const value: ResourceGroupsContext = useMemo(() => {
    function toggleGroupExpanded(groupLabel: string) {
      const currentGroupState = groups[groupLabel] ?? { ...DEFAULT_GROUP_STATE }
      const nextGroupState = {
        ...currentGroupState,
        expanded: !currentGroupState.expanded,
      }

      setGroups((prevState) => {
        return {
          ...prevState,
          [groupLabel]: nextGroupState,
        }
      })
    }

    function getGroup(groupLabel: string) {
      return groups[groupLabel] ?? { ...DEFAULT_GROUP_STATE }
    }

    // We expand all groups by resetting the collapse state to empty.
    //
    // This ensures that even hidden groups are expanded.
    //
    // NOTE(nick): expandAll and collapseAll are non-symmetric - they have
    // very different behavior for groups that are currently hidden, or
    // for new groups created after the button is clicked. We deliberately
    // err on the side of expanding.
    function expandAll() {
      setGroups({}) // Reset state.
    }

    // We can collapse all groups currently on-screen.
    //
    // If new resources with new names come later, we'll leave them expanded.
    function collapseAll(groupNames: string[]) {
      let newState: GroupsState = {}
      groupNames.forEach((group) => (newState[group] = { expanded: false }))
      setGroups(newState)
    }

    return {
      groups,
      toggleGroupExpanded,
      getGroup,
      expandAll,
      collapseAll,
    }
  }, [groups, setGroups])

  return (
    <resourceGroupsContext.Provider value={value}>
      {props.children}
    </resourceGroupsContext.Provider>
  )
}
