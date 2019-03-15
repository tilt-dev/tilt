import React, { Component } from 'react';
import './ResourceList.scss';
import './text.scss';

function ResourceList(props) {
  let children = props.resources.map((resource) => {
    return <ResourceSummary key={resource.Name} resource={resource} openPreview={endpoint => props.openPreview(endpoint)}/>
  })

  return (
    <section className="resources" role="table">
      <header className="headings">
        <p role="columnheader" className="column-header">Resource Name</p>
        <p role="columnheader" className="column-header">K8S</p>
        <p role="columnheader" className="column-header">Build Status</p>
        <p role="columnheader" className="column-header">Updated</p>
        <p role="columnheader" className="column-header">Endpoint</p>
      </header>
      <ul>
        {children}
      </ul>
    </section>
  )
}

class ResourceSummary extends Component {
  render() {
    let resource = this.props.resource
    let k8sStatus = getK8sStatus(resource)
    let buildStatus = getBuildStatus(resource)
    let updateTime = getUpdateTime(resource)
    let endpoint = getEndpoint(resource)
    let endpointEl

    if (endpoint) {
      endpointEl = <React.Fragment>
        <a className="endpoint" href={endpoint} title={`Open in new window: ${endpoint}`} target="_blank" rel="noopener noreferrer">{endpoint}</a>
        <button onClick={() => this.props.openPreview(endpoint)}>Preview</button>
      </React.Fragment>
    } else {
      endpointEl = "—"
    }


    return (
      <li role="rowgroup" className="resource">
        <p role="cell" className="cell name">{resource.Name}</p>
        <p role="cell" className="cell">{k8sStatus}</p>
        <p role="cell" className="cell">{buildStatus}</p>
        <p role="cell" className="cell">{updateTime}</p>
        <p role="cell" className="cell">{endpointEl}</p>
      </li>
    )
  }
}

let zeroTime = "0001-01-01T00:00:00Z"

function isZeroTime(t) {
  return !t || t === zeroTime
}

function isZeroBuildStatus(s) {
  return isZeroTime(s.StartTime)
}

function lastBuild(res) {
  if (!res.BuildHistory || !res.BuildHistory.length) {
    return {}
  }
  return res.BuildHistory[0]
}

function getK8sStatus(res) {
  if (res.IsYAMLManifest) {
    return "—"
  }
  return (res.ResourceInfo && res.ResourceInfo.PodStatus) || "Pending"
}

function getBuildStatus(res) {
  let status = "Pending"
  if (!isZeroBuildStatus(res.CurrentBuild)) {
    status = "Building"
  } else if (!isZeroTime(res.PendingBuildSince)) {
    status = "Pending"
  } else if (!isZeroBuildStatus(lastBuild(res))) {
    let last = lastBuild(res)
    if (last.Error) {
      status = "Error"
    } else {
      status = "OK"
    }
  }
  return status
}

function getUpdateTime(res) {
  let time = res.LastDeployTime
  if (isZeroTime(time)) {
    return "—"
  }

  return new Date(time).toLocaleTimeString('en-US')
}

function getEndpoint(res) {
  return res.Endpoints && res.Endpoints[0]
}

export default ResourceList;
