// Functions for interacting with snapshot UI elements.

import React, { PropsWithChildren, useContext, useMemo } from "react"
import { Flag, useFeatures } from "./feature"
import { usePathBuilder } from "./PathBuilder"

export type SnapshotAction = {
  enabled: boolean
  openModal: (dialogAnchor?: HTMLElement | null) => void
}

const snapshotActionContext = React.createContext<SnapshotAction>({
  enabled: true,
  openModal: (dialogAnchor?: HTMLElement | null) => {},
})

export function useSnapshotAction(): SnapshotAction {
  return useContext(snapshotActionContext)
}

export function SnapshotActionProvider(
  props: PropsWithChildren<{
    openModal: (dialogAnchor?: HTMLElement | null) => void
  }>
) {
  let openModal = props.openModal
  let features = useFeatures()
  let pathBuilder = usePathBuilder()
  let showSnapshot =
    features.isEnabled(Flag.Snapshots) && !pathBuilder.isSnapshot()

  let snapshotAction = useMemo(() => {
    return {
      enabled: showSnapshot,
      openModal: openModal,
    }
  }, [showSnapshot, openModal])

  return (
    <snapshotActionContext.Provider value={snapshotAction}>
      {props.children}
    </snapshotActionContext.Provider>
  )
}

export let SnapshotActionTestProvider = snapshotActionContext.Provider
