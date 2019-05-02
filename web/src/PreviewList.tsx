import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import PathBuilder from "./PathBuilder"
import { Link } from "react-router-dom"
import "./PreviewList.scss"

type PreviewListProps = {
  pathBuilder: PathBuilder
  resourcesWithEndpoints: Array<string>
}

class PreviewList extends PureComponent<PreviewListProps> {
  render() {
    let pb = this.props.pathBuilder

    let endpointsEmptyEl = (
      <section className="Pane-empty-message">
        <LogoWordmarkSvg />
        <h2>No Endpoints Found</h2>
        <p>
          If you'd like to preview your resources in the browser, <br />
          the <a href="https://docs.tilt.dev/tutorial.html">
            Tilt Tutorial
          </a>{" "}
          has more on setting up port forwarding.
        </p>
      </section>
    )

    let endpointsEl = (
      <ul>
        {this.props.resourcesWithEndpoints.map(r => (
          <li key={r}>
            <Link to={pb.path(`/r/${r}/preview`)} className="PreviewList-link">
              {r}
            </Link>
          </li>
        ))}
      </ul>
    )

    let noEndpoints = this.props.resourcesWithEndpoints.length === 0

    return (
      <section
        className={`PreviewList ${noEndpoints ? "PreviewList--empty" : ""}`}
      >
        {noEndpoints ? endpointsEmptyEl : endpointsEl}
      </section>
    )
  }
}

export default PreviewList
