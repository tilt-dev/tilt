import React, { PureComponent } from "react"
import "./ResourceInfo.scss"

type ResourceInfoProps = {
  podID: string
  endpoints: string[]
  podStatus: string
}

class ResourceInfo extends PureComponent<ResourceInfoProps> {
  render() {
    let podID = this.props.podID
    let podStatus = this.props.podStatus
    let podIDEl = podID && (
      <>
        <div className="resourceInfo">
          <div className="resourceInfo-label">Pod Status:</div>
          <div className="resourceInfo-value">{podStatus}</div>
        </div>
        <div className="resourceInfo">
          <div className="resourceInfo-label">Pod ID:</div>
          <div className="resourceInfo-value">{podID}</div>
        </div>
      </>
    )

    let endpoints = this.props.endpoints
    let endpointsEl = endpoints.length > 0 && (
      <div className="resourceInfo">
        <div className="resourceInfo-label">
          Endpoint{endpoints.length > 1 ? "s" : ""}:
        </div>
        {endpoints.map(ep => (
          <a
            className="resourceInfo-value"
            href={ep}
            target="_blank"
            rel="noopener noreferrer"
            key={ep}
          >
            {ep}
          </a>
        ))}
      </div>
    )

    if (endpoints || podID)
      return (
        <section className="resourceBar">
          {podIDEl}
          {endpointsEl}
        </section>
      )
  }
}

export default ResourceInfo
