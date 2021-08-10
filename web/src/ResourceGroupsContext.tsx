import react, { PropsWithChildren, useContext, useState } from 'react'
import { createContext } from 'react'

// To use local storage, you can use the `usePersistentState` hook instead of `useState`
// I'm not sure how the provider or context ingestion is different, but I think everything else is the same

export enum GroupView {
  Expanded = 'expanded',
  Collapsed = 'collapsed',
}

export const DEFAULT_GROUP_VIEW = GroupView.Expanded

export type ResourceGroupsState = { [key: string]: GroupView }

/**
 * Alternatively, this could be `collapsedGroups` where values are boolean.
 * I'm slightly concerned with, when first seeing a label group and not having any
 * saved state, then updating the state and causing a re-render. But if it's an
 * okay design pattern to only update the state once a group has been collapsed,
 * then it should be fine.
 */

type ResourceGroupsContext = {
  groups: ResourceGroupsState;
  setGroup: (groupLabel: string) => void;
  getGroup: (groupLabel: string) => GroupView;
}

const resourceGroupsContext = createContext<ResourceGroupsContext>({
  groups: {},
  setGroup: () => {},
  getGroup: () => DEFAULT_GROUP_VIEW
})

export function useResourceGroups(): ResourceGroupsContext {
  return useContext(resourceGroupsContext)
}

export function ResourceGroupsContextProvider(props: PropsWithChildren<{}>) {

  const [ groups, setGroupsState ] = useState<ResourceGroupsState>({})

  function setGroup(groupLabel: string) {
    const currentGroupState: GroupView = groups[groupLabel] ?? DEFAULT_GROUP_VIEW
    const nextGroupState: GroupView = currentGroupState === GroupView.Expanded ? GroupView.Collapsed : GroupView.Expanded

    // TODO: Move analytics call here

    setGroupsState((prevState) => ({ ...prevState, [groupLabel]: nextGroupState }))
  }

  function getGroup(groupLabel: string) {
    return groups[groupLabel] ?? DEFAULT_GROUP_VIEW
  }

  const resourceGroupDefaultValue = { groups: {}, setGroup, getGroup }

  return (
    <resourceGroupsContext.Provider value={resourceGroupDefaultValue}>
      {props.children}
    </resourceGroupsContext.Provider>
  )
}
