import React, { useMemo } from "react"
import styled from "styled-components"
import { ApiButtonToggleState, buttonsByComponent } from "./ApiButton"
import { BulkApiButton } from "./BulkApiButton"
import { Flag, useFeatures } from "./feature"
import { useResourceSelection } from "./ResourceSelectionContext"
import SrOnly from "./SrOnly"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { UIButton } from "./types"

type OverviewTableBulkActionsProps = {
  uiButtons?: UIButton[]
}

type ActionButtons = { [key in BulkAction]: UIButton[] }

export enum BulkAction {
  Disable = "disable", // Enable / disable are states of the same toggle, so use a single name
}

const BulkActionMenu = styled.div`
  align-items: center;
  display: flex;
  flex-direction: row;
  margin-left: ${SizeUnit(2)};
  white-space: nowrap;
`

const SelectedCount = styled.p`
  margin: ${SizeUnit(0.25)};
  font-size: ${FontSize.small};
  color: ${Color.gray7};
`

function BulkSelectedCount({ count }: { count: number }) {
  if (!count) {
    return null
  }

  return (
    <SelectedCount>
      {count} <SrOnly>resources</SrOnly> selected
    </SelectedCount>
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

  // TODO (lizz): Do more testing to investigate "memory leak" warning
  // from unmounting this component when its `setState` hooks haven't
  // finished running

  // Don't render the bulk actions is feature flag is off
  // or if there are no selections
  if (!features.isEnabled(Flag.BulkDisableResources) || selected.length === 0) {
    return null
  }

  const onClickCallback = () => clearSelections()

  return (
    <BulkActionMenu>
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Enable"
        className="firstButtonGroupInRow"
        requiresConfirmation={false}
        uiButtons={actionButtons[BulkAction.Disable]}
        targetToggleState={ApiButtonToggleState.On}
        onClickCallback={onClickCallback}
      />
      <BulkApiButton
        bulkAction={BulkAction.Disable}
        buttonText="Disable"
        className="lastButtonGroupInRow"
        requiresConfirmation={true}
        uiButtons={actionButtons[BulkAction.Disable]}
        targetToggleState={ApiButtonToggleState.Off}
        onClickCallback={onClickCallback}
      />
      <BulkSelectedCount count={selected.length} />
    </BulkActionMenu>
  )
}
