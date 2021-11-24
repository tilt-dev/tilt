import { createContext, PropsWithChildren, useContext, useEffect } from "react"
import { AnalyticsAction, AnalyticsType, incr, Tags } from "./analytics"
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

function getTags(groups: GroupsState): Tags {
  let expandCount = 0
  let collapseCount = 0
  Object.values(groups).forEach((group) => {
    if (group.expanded) {
      expandCount++
    } else {
      collapseCount++
    }
  })
  return {
    expanded: String(expandCount),
    collapsed: String(collapseCount),
  }
}

export function ResourceGroupsContextProvider(
  props: PropsWithChildren<{ initialValuesForTesting?: GroupsState }>
) {
  const defaultPersistentValue = props.initialValuesForTesting ?? {}
  const [groups, setGroups] = usePersistentState<GroupsState>(
    "resource-groups",
    defaultPersistentValue
  )
  let analyticsTags = getTags(groups)

  useEffect(() => {
    incr("ui.web.resourceGroup", {
      action: AnalyticsAction.Load,
      ...analyticsTags,
    })
    // empty deps because we only want to report nce per app load
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function toggleGroupExpanded(groupLabel: string, page: AnalyticsType) {
    const currentGroupState = groups[groupLabel] ?? { ...DEFAULT_GROUP_STATE }
    const nextGroupState = {
      ...currentGroupState,
      expanded: !currentGroupState.expanded,
    }

    const action = nextGroupState.expanded
      ? AnalyticsAction.Expand
      : AnalyticsAction.Collapse
    incr("ui.web.resourceGroup", {
      action,
      type: page,
      ...analyticsTags,
    })

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
