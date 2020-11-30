import React from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"

type Serving = Proto.webviewMetricsServing

let MetricsPaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  padding: 16px;
`

let MetricsHeader = styled.a`
  display: block;
  margin-left: 16px;
  font-size: 22px;
`

let MetricsGraphRoot = styled.div`
  display: flex;
`

let MetricsGraph = styled.iframe`
  display: block;
  width: 30vw;
  height: 20vw;
  background: transparent;
  border: none;
  margin: 16px;
`

function MetricsPane(props: { pathBuilder: PathBuilder; serving: Serving }) {
  let protocol = props.pathBuilder.isSecure() ? "https" : "http"
  let root = `${protocol}://${props.serving.grafanaHost}`

  let frames = [
    "/d-solo/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s&panelId=2",
    "/d-solo/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s&panelId=3",
  ]

  let graphs = frames.map((frame, i) => {
    return <MetricsGraph key={"graph" + i} src={root + frame}></MetricsGraph>
  })

  let link = `${root}/d/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s`

  return (
    <MetricsPaneRoot>
      <MetricsHeader href={link}>Full Dashboard</MetricsHeader>
      <MetricsGraphRoot>{graphs}</MetricsGraphRoot>
    </MetricsPaneRoot>
  )
}

export default MetricsPane
