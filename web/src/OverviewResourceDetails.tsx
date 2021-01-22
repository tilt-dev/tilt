import React from "react"
import { useHistory } from "react-router-dom"
import styled from "styled-components"
import { filterSetFromLocation } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import OverviewLogPane from "./OverviewLogPane"
import { Color } from "./style-helpers"
import { ResourceName } from "./types"

type OverviewResourceDetailsProps = {
  resource?: Proto.webviewResource
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
  let { name, resource } = props
  let manifestName = resource?.name || ""
  let all = name === "" || name === ResourceName.all
  let notFound = !all && !manifestName
  let history = useHistory()
  let filterSet = filterSetFromLocation(history.location)

  return (
    <OverviewResourceDetailsRoot>
      <OverviewActionBar resource={resource} />
      {notFound ? (
        <NotFound>No resource '{name}'</NotFound>
      ) : (
        <OverviewLogPane manifestName={manifestName} filterSet={filterSet} />
      )}
    </OverviewResourceDetailsRoot>
  )
}
