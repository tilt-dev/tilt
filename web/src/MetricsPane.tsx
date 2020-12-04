import React from "react"
import styled from "styled-components"
import ButtonInput from "./ButtonInput"
import PathBuilder from "./PathBuilder"
import { Font, FontSize } from "./style-helpers"

type Serving = Proto.webviewMetricsServing

let MetricsPaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  padding: 32px;
`

let MetricsHeader = styled.div`
  display: flex;
  margin: 16px;
  font-size: ${FontSize.large};
  font-family: ${Font.sansSerif};
  justify-content: space-between;
  align-items: center;
`

let MetricsDescription = styled.div`
  margin: 16px;
  font-size: ${FontSize.small};
`

let MetricsGraphRoot = styled.div`
  display: flex;
`

let MetricsButtonBlock = styled(ButtonInput)`
  width: auto;
  margin: 16px auto 16px 16px;
`
let MetricsButtonRight = styled(ButtonInput)`
  width: auto;
  margin: 16px;
`

let MetricsGraph = styled.iframe`
  display: block;
  width: 30vw;
  height: 20vw;
  background: transparent;
  border: none;
  margin: 16px;
`

function enableLocalMetrics(opt: string) {
  fetch(`/api/metrics_opt`, {
    method: "post",
    body: opt,
  })
}

function MetricsTeaser() {
  return (
    <MetricsPaneRoot>
      <MetricsHeader>Metrics</MetricsHeader>
      <MetricsDescription>
        Experimental: Enabling this pane deploys a small metrics stack to your
        cluster that monitors your build performance.
        <p />
        These metrics are not sent outside your cluster.
        <p />
        We'd love to{" "}
        <a
          href="https://docs.tilt.dev/#community"
          target="_blank"
          rel="noopener noreferrer"
        >
          hear from you
        </a>{" "}
        on your thoughts on this feature.
        <p />
      </MetricsDescription>

      <MetricsButtonBlock
        type="button"
        value="Enable Metrics"
        onClick={() => enableLocalMetrics("local")}
      />
    </MetricsPaneRoot>
  )
}

function MetricsLoading() {
  return (
    <MetricsPaneRoot>
      <MetricsHeader>
        <div>Loading dashboards...</div>
        <MetricsButtonRight
          type="button"
          value="Disable Metrics"
          onClick={() => enableLocalMetrics("disabled")}
        />
      </MetricsHeader>
    </MetricsPaneRoot>
  )
}

function MetricsPane(props: { pathBuilder: PathBuilder; serving: Serving }) {
  if (props.serving.mode !== "local") {
    return <MetricsTeaser />
  }

  if (!props.serving.grafanaHost) {
    return <MetricsLoading />
  }

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
      <MetricsHeader>
        <a href={link} target="_blank" rel="noopener noreferrer">
          Full Dashboard
        </a>
        <MetricsButtonRight
          type="button"
          value="Disable Metrics"
          onClick={() => enableLocalMetrics("")}
        />
      </MetricsHeader>
      <MetricsGraphRoot>{graphs}</MetricsGraphRoot>
    </MetricsPaneRoot>
  )
}

export default MetricsPane
