import React, { PureComponent } from "react"
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
    if (this.props.resourcesWithEndpoints.length === 0) {
      return <p>No resources found with endpoints!</p>
    }
    let links = this.props.resourcesWithEndpoints.map(r => (
      <li key={r}>
        <Link to={pb.path(`/r/${r}/preview`)}>{r}</Link>
      </li>
    ))

    return (
      <section className="PreviewList">
        <p>The following resources have preview endpoints:</p>
        <ul>{links}</ul>
      </section>
    )
  }
}

export default PreviewList
