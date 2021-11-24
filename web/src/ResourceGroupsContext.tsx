import { createContext, PropsWithChildren, useContext } from "react"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { usePersistentState } from "./LocalStorage"

export type GroupState = { expanded: boolean }

export type GroupsState = {
  [key: string]: GroupState
}

type ResourceGroupsContext = {
  groups: GroupsState
  getGroup: (groupLabel: string) => GroupState
  toggleGroupExpanded: (groupLabel: string, page: AnalyticsType) => void
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

  const defaultValue: ResourceGroupsContext = {
    groups,
    toggleGroupExpanded,
    getGroup,
  }

  return (
    <resourceGroupsContext.Provider value={defaultValue}>
      {props.children}
    </resourceGroupsContext.Provider>
  )
}
