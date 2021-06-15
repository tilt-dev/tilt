import React from "react"
import styled from "styled-components"
import { Alert } from "./alerts"
import { useFilterSet } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import OverviewLogPane from "./OverviewLogPane"
import { Color } from "./style-helpers"
import { ResourceName } from "./types"

type UIResource = Proto.v1alpha1UIResource
type UIButton = Proto.v1alpha1UIButton

type OverviewResourceDetailsProps = {
  resource?: UIResource
  buttons?: UIButton[]
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
  background-color: ${Color.grayDarkest};
`

export default function OverviewResourceDetails(
  props: OverviewResourceDetailsProps
) {
  let { name, resource, alerts, buttons } = props
  let manifestName = resource?.metadata?.name || ""
  let all = name === "" || name === ResourceName.all
  let notFound = !all && !manifestName
  let filterSet = useFilterSet()

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
        <OverviewLogPane manifestName={manifestName} filterSet={filterSet} />
      )}
    </OverviewResourceDetailsRoot>
  )
}
