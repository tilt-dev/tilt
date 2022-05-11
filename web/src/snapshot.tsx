// Functions for interacting with snapshot UI elements.

import React, { PropsWithChildren, useContext, useMemo } from "react"
import { Flag, useFeatures } from "./feature"
import { usePathBuilder } from "./PathBuilder"

export type SnapshotAction = {
  enabled: boolean
  openModal: (dialogAnchor?: HTMLElement | null) => void
  currentSnapshotTime?: {
    tiltUpTime?: string
    createdAt?: string
  }
}

export type SnapshotProviderProps = Pick<
  SnapshotAction,
  "openModal" | "currentSnapshotTime"
>

const snapshotActionContext = React.createContext<SnapshotAction>({
  enabled: true,
  openModal: (dialogAnchor?: HTMLElement | null) => {},
})

export function useSnapshotAction(): SnapshotAction {
  return useContext(snapshotActionContext)
}

export function SnapshotActionProvider(
  props: PropsWithChildren<SnapshotProviderProps>
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
      currentSnapshotTime: props.currentSnapshotTime,
    }
  }, [showSnapshot, openModal])

  return (
    <snapshotActionContext.Provider value={snapshotAction}>
      {props.children}
    </snapshotActionContext.Provider>
  )
}

export let SnapshotActionTestProvider = snapshotActionContext.Provider
