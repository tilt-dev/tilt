import { createContext, PropsWithChildren, useContext } from "react"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { usePersistentState } from "./LocalStorage"

const GROUPS_LOCAL_STORAGE_NAME = "resource-groups"
const DEFAULT_GROUP_STATE = true

export type GroupExpandedState = { [key: string]: boolean }

type ResourceGroupsContext = {
  expanded: GroupExpandedState
  setGroup: (groupLabel: string, page: AnalyticsType) => void
  getGroup: (groupLabel: string) => boolean
}

const resourceGroupsContext = createContext<ResourceGroupsContext>({
  expanded: {},
  setGroup: () => {
    // Note: this warning should only show in development
    console.warn(
      "Resource group context is not set. Did you forget to provide it?"
    )
  },
  getGroup: () => {
    // Note: this warning should only show in development
    console.warn(
      "Resource group context is not set. Did you forget to provide it?"
    )
    return DEFAULT_GROUP_STATE
  },
})

export function useResourceGroups(): ResourceGroupsContext {
  return useContext(resourceGroupsContext)
}

export function ResourceGroupsProvider(props: PropsWithChildren<{}>) {
  const [expanded, setExpandedState] = usePersistentState<GroupExpandedState>(
    GROUPS_LOCAL_STORAGE_NAME,
    {}
  )

  function setGroup(groupLabel: string, page: AnalyticsType) {
    const currentGroupState = expanded[groupLabel] ?? DEFAULT_GROUP_STATE
    const nextGroupState = !currentGroupState

    const action = nextGroupState
      ? AnalyticsAction.Expand
      : AnalyticsAction.Collapse
    incr("ui.web.resourceGroup", { action, type: page })

    setExpandedState((prevState) => ({
      ...prevState,
      [groupLabel]: nextGroupState,
    }))
  }

  function getGroup(groupLabel: string) {
    return expanded[groupLabel] ?? DEFAULT_GROUP_STATE
  }

  const defaultValue = { expanded: {}, setGroup, getGroup }

  return (
    <resourceGroupsContext.Provider value={defaultValue}>
      {props.children}
    </resourceGroupsContext.Provider>
  )
}
