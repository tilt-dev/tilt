import React, { Component } from 'react';
import './ResourceList.css';
import './text.css';

function ResourceList(props) {
  let children = props.resources.map((resource) => {
    return <ResourceSummary key={resource.Name} resource={resource} />
  })

  return (
    <div className="ResourceList">
      <div className="ResourceList-header">
        <div className="Resource-lhsCell u-muted">Resource Name</div>
        <div className="Resource-spacerCell">&nbsp;</div>
        <div className="Resource-rhsCell u-muted">K8S</div>
        <div className="u-muted">&nbsp;•&nbsp;</div>
        <div className="Resource-rhsCell u-muted">Build Status</div>
        <div className="u-muted">&nbsp;•&nbsp;</div>
        <div className="Resource-rhsCell u-muted">Updated</div>
      </div>
      {children}
    </div>
  )
}

class ResourceSummary extends Component {
  render() {
    let resource = this.props.resource
    let k8sStatus = getK8sStatus(resource)
    let buildStatus = getBuildStatus(resource)
    let updateTime = getUpdateTime(resource)
    return (
      <div className="ResourceSummary">
        <div className="Resource-lhsCell Resource-name">{resource.Name}</div>
        <div className="Resource-spacerCell">&nbsp;</div>
        <div className="Resource-rhsCell">{k8sStatus}</div>
        <div>&nbsp;•&nbsp;</div>
        <div className="Resource-rhsCell">{buildStatus}</div>
        <div>&nbsp;•&nbsp;</div>
        <div className="Resource-rhsCell">{updateTime}</div>
      </div>
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
    return "-"
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
    return "-"
  }

  return new Date(time).toLocaleTimeString('en-US')
}

export default ResourceList;
