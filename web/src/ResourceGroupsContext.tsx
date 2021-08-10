import { createContext, PropsWithChildren, useContext, useState } from "react"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"

// To use local storage, you can use the `usePersistentState` hook instead of `useState`
// I'm not sure how the provider or context ingestion is different, but I think everything else is the same

export const DEFAULT_GROUP_STATE = true

export type GroupExpandedState = { [key: string]: boolean }

type ResourceGroupsContext = {
  expanded: GroupExpandedState
  setGroup: (groupLabel: string, page: AnalyticsType) => void
  getGroup: (groupLabel: string) => boolean
}

const resourceGroupsContext = createContext<ResourceGroupsContext>({
  expanded: {},
  setGroup: () => {
    console.warn("Resource group context is not set.")
  },
  getGroup: () => {
    console.warn("Resource group context is not set.")
    return DEFAULT_GROUP_STATE
  },
})

export function useResourceGroups(): ResourceGroupsContext {
  return useContext(resourceGroupsContext)
}

export function ResourceGroupsContextProvider(props: PropsWithChildren<{}>) {
  const [expanded, setExpandedState] = useState<GroupExpandedState>({})

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
