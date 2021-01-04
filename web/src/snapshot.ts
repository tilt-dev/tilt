// Functions for interacting with snapshot UI elements.

import React, { useContext } from "react"

export type SnapshotAction = {
  enabled: boolean
  openModal: () => void
}

const snapshotActionContext = React.createContext<SnapshotAction>({
  enabled: true,
  openModal: () => {},
})

export function useSnapshotAction(): SnapshotAction {
  return useContext(snapshotActionContext)
}

export let SnapshotActionProvider = snapshotActionContext.Provider
