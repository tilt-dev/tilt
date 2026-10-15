import React, { useEffect, useRef } from "react"
import styled from "styled-components"
import { Alert } from "./alerts"
import { ButtonSet } from "./ApiButton"
import LogDetailsDrawer from "./LogDetailsDrawer"
import { useFilterSet } from "./logfilters"
import { useSelectedLogLine } from "./logselection"
import { useLogStore } from "./LogStore"
import OverviewActionBar from "./OverviewActionBar"
import OverviewLogPane from "./OverviewLogPane"
import { Color } from "./style-helpers"
import { ResourceName, UIResource } from "./types"

type OverviewResourceDetailsProps = {
  resource?: UIResource
  buttons?: ButtonSet
  alerts?: Alert[]
  name: string
}

let OverviewResourceDetailsRoot = styled.div`
  display: flex;
  flex-grow: 1;
  flex-shrink: 1;
  flex-direction: column;
`

// Positions the log details drawer over the log pane.
let LogPaneContainer = styled.div`
  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
`

let NotFound = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: ${Color.gray10};
`

export default function OverviewResourceDetails(
  props: OverviewResourceDetailsProps
) {
  let { name, resource, alerts, buttons } = props
  let manifestName = resource?.metadata?.name || ""
  let all = name === "" || name === ResourceName.all
  let starred = name === "" || name === ResourceName.starred
  let notFound = !all && !starred && !manifestName
  let filterSet = useFilterSet()
  let logStore = useLogStore()
  let [selectedLineIndex, setSelectedLineIndex] = useSelectedLogLine()

  // The selected line belongs to the resource it was selected in,
  // so clear it when switching resources.
  let prevName = useRef(name)
  useEffect(() => {
    if (prevName.current !== name) {
      prevName.current = name
      setSelectedLineIndex(null)
    }
  }, [name, setSelectedLineIndex])

  return (
    <OverviewResourceDetailsRoot>
      <OverviewActionBar
        resource={resource}
        filterSet={filterSet}
        alerts={alerts}
        buttons={buttons}
      />
      {notFound ? (
        <NotFound>No resource '{name}'</NotFound>
      ) : (
        <LogPaneContainer>
          <OverviewLogPane
            manifestName={starred ? name : manifestName}
            filterSet={filterSet}
          />
          {selectedLineIndex !== null && (
            <LogDetailsDrawer
              logStore={logStore}
              selectedLineIndex={selectedLineIndex}
              onClose={() => setSelectedLineIndex(null)}
            />
          )}
        </LogPaneContainer>
      )}
    </OverviewResourceDetailsRoot>
  )
}
