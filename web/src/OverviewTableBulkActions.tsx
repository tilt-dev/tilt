import React, { useMemo } from "react"
import { ApiButtonToggleState, buttonsByComponent } from "./ApiButton"
import { BulkApiButton } from "./BulkApiButton"
import { Flag, useFeatures } from "./feature"
import { useResourceSelection } from "./ResourceSelectionContext"
import SrOnly from "./SrOnly"
import { UIButton } from "./types"

type OverviewTableBulkActionsProps = {
  uiButtons?: UIButton[]
}

type ActionButtons = { [key in BulkAction]: UIButton[] }

export enum BulkAction {
  Disable = "disable", // Enable / disable are states of the same toggle, so use a single name
}

function SelectedCount({ count }: { count: number }) {
  if (!count) {
    return null
  }

  return (
    <p>
      {count} <SrOnly>resources</SrOnly> selected
    </p>
  )
}

export function OverviewTableBulkActions({
  uiButtons,
}: OverviewTableBulkActionsProps) {
  const features = useFeatures()
  const { selected, clearSelections } = useResourceSelection()

  const resourceButtons = useMemo(
    () => buttonsByComponent(uiButtons),
    [uiButtons]
  )

  const actionButtons = useMemo(() => {
    const buttonsByAction: ActionButtons = {
      [BulkAction.Disable]: [],
    }

    selected.forEach((resource) => {
      const buttonSet = resourceButtons[resource]
      if (buttonSet && buttonSet.toggleDisable) {
        buttonsByAction[BulkAction.Disable].push(buttonSet.toggleDisable)
      }
    })

    return buttonsByAction
  }, [selected, uiButtons])

  // Don't render the bulk actions is feature flag is off
  if (!features.isEnabled(Flag.BulkDisableResources)) {
    return null
  }

  const onClickCallback = () => clearSelections()

  return (
    <>
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Enable"
        requiresConfirmation={false}
        uiButtons={actionButtons[BulkAction.Disable]}
        targetToggleState={ApiButtonToggleState.On}
        onClickCallback={onClickCallback}
      />
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Disable"
        requiresConfirmation={true}
        uiButtons={actionButtons[BulkAction.Disable]}
        targetToggleState={ApiButtonToggleState.Off}
        onClickCallback={onClickCallback}
      />
      <SelectedCount count={selected.length} />
    </>
  )
}
