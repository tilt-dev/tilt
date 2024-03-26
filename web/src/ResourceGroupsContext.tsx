import { createContext, PropsWithChildren, useContext, useMemo } from "react"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { usePersistentState } from "./BrowserStorage"

export type GroupState = { expanded: boolean }

export type GroupsState = {
  [key: string]: GroupState
}

type ResourceGroupsContext = {
  groups: GroupsState
  getGroup: (groupLabel: string) => GroupState
  toggleGroupExpanded: (groupLabel: string, page: AnalyticsType) => void
  expandAll: (groups: string[]) => void
  collapseAll: (groups: string[]) => void
}

export const DEFAULT_EXPANDED_STATE = false
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
  expandAll: (groups: string[]) => void 0,
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
    function toggleGroupExpanded(groupLabel: string, page: AnalyticsType) {
      const currentGroupState = groups[groupLabel] ?? { ...DEFAULT_GROUP_STATE }
      const nextGroupState = {
        ...currentGroupState,
        expanded: !currentGroupState.expanded,
      }

      const action = nextGroupState.expanded
        ? AnalyticsAction.Expand
        : AnalyticsAction.Collapse
      incr("ui.web.resourceGroup", { action, type: page })

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

    function setAllExpanded(groupNames: string[], newValue: boolean) {
      if (newValue == DEFAULT_EXPANDED_STATE) {
        setGroups({}) // Reset state.
      } else {
        let newState: GroupsState = {}
        groupNames.forEach(
          (group) => (newState[group] = { expanded: newValue })
        )
        setGroups(newState)
      }
    }

    // We expand all groups currently on-screen.
    //
    // If new resources with new names come later, we'll collapse them.
    function expandAll(groupNames: string[]) {
      setAllExpanded(groupNames, true)
    }

    // We can collapse all groups currently on-screen.
    function collapseAll(groupNames: string[]) {
      setAllExpanded(groupNames, false)
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
