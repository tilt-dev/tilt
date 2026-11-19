import React, { useEffect, useState } from "react"
import styled from "styled-components"
import { Alert, combinedAlerts } from "./alerts"
import { ButtonSet } from "./ApiButton"
import { useFilterSet } from "./logfilters"
import { LogUpdateAction, LogUpdateEvent, useLogStore } from "./LogStore"
import OverviewActionBar from "./OverviewActionBar"
import OverviewLogPane from "./OverviewLogPane"
import { Color } from "./style-helpers"
import { ResourceName, UIResource } from "./types"

type OverviewResourceDetailsProps = {
  resource?: UIResource
  buttons?: ButtonSet
  alerts?: Alert[]
  name: string
  // Passed through to the action bar; see OverviewActionBar.
  logControlsOnly?: boolean
}

let OverviewResourceDetailsRoot = styled.div`
  display: flex;
  flex-grow: 1;
  flex-shrink: 1;
  flex-direction: column;
  min-height: 0;
`

let NotFound = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: ${Color.gray10};
`

// Alerts for the given resources. Rebuilt whenever the logs are cleared.
export function useResourceAlerts(resources: UIResource[]): Alert[] {
  const logStore = useLogStore()
  const [truncateCount, setTruncateCount] = useState<number>(0)

  // add a listener to rebuild alerts whenever a truncation event occurs
  // truncateCount is a dummy state variable to trigger a re-render to
  // simplify logic vs reconciliation between logStore + props
  useEffect(() => {
    const rebuildAlertsOnLogClear = (e: LogUpdateEvent) => {
      if (e.action === LogUpdateAction.truncate) {
        setTruncateCount(truncateCount + 1)
      }
    }

    logStore.addUpdateListener(rebuildAlertsOnLogClear)
    return () => logStore.removeUpdateListener(rebuildAlertsOnLogClear)
  }, [truncateCount])

  let alerts: Alert[] = []
  resources.forEach((r) => alerts.push(...combinedAlerts(r, logStore)))
  return alerts
}

export default function OverviewResourceDetails(
  props: OverviewResourceDetailsProps
) {
  let { name, resource, alerts, buttons } = props
  let manifestName = resource?.metadata?.name || ""
  let all = name === "" || name === ResourceName.all
  let starred = name === "" || name === ResourceName.starred
  let notFound = !all && !starred && !manifestName
  let filterSet = useFilterSet()

  return (
    <OverviewResourceDetailsRoot>
      <OverviewActionBar
        resource={resource}
        filterSet={filterSet}
        alerts={alerts}
        buttons={buttons}
        logControlsOnly={props.logControlsOnly}
      />
      {notFound ? (
        <NotFound>No resource '{name}'</NotFound>
      ) : (
        <OverviewLogPane
          manifestName={starred ? name : manifestName}
          filterSet={filterSet}
        />
      )}
    </OverviewResourceDetailsRoot>
  )
}
