import React, { PureComponent } from "react"
import "./PreviewPane.scss"

type PreviewProps = {
  endpoint: string
  isExpanded: boolean
}

class PreviewPane extends PureComponent<PreviewProps> {
  render() {
    let classes = `
      PreviewPane
      ${this.props.isExpanded ? "PreviewPane--expanded" : ""}
      ${this.props.endpoint ? "" : "PreviewPane-empty"}
    `

    let content
    if (this.props.endpoint) {
      content = <iframe src={this.props.endpoint} />
    } else {
      content = (
        <section>
          <h2>No endpoint found</h2>
          <p>
            If this is a resource that can be previewed in the browser, the{" "}
            <a href="https://docs.tilt.dev/tutorial.html">Tilt Tutorial</a> has
            more on setting up port forwarding.
          </p>
        </section>
      )
    }

    return <section className={classes}>{content}</section>
  }
}

export default PreviewPane
