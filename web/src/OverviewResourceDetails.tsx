import React, { useEffect, useState } from "react"
import styled from "styled-components"
import { Alert } from "./alerts"
import { ButtonSet } from "./ApiButton"
import { useFilterSet } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import OverviewLogPane from "./OverviewLogPane"
import { useLogStore } from "./LogStore"
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

  const logStore = useLogStore()
  const logTarget = starred ? name : manifestName
  const [logContainers, setLogContainers] = useState<string[]>(() =>
    logStore.containersForManifest(logTarget)
  )

  useEffect(() => {
    const onUpdate = () => {
      const next = logStore.containersForManifest(logTarget)
      setLogContainers((prev) =>
        prev.length === next.length && prev.every((c, i) => c === next[i])
          ? prev
          : next
      )
    }
    onUpdate()
    logStore.addUpdateListener(onUpdate)
    return () => logStore.removeUpdateListener(onUpdate)
  }, [logStore, logTarget])

  return (
    <OverviewResourceDetailsRoot>
      <OverviewActionBar
        resource={resource}
        filterSet={filterSet}
        alerts={alerts}
        buttons={buttons}
        logContainers={logContainers}
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
