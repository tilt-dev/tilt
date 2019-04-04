import React, { PureComponent } from "react"
import "./PreviewPane.scss"

type PreviewProps = {
  endpoint: string
}

class PreviewPane extends PureComponent<PreviewProps> {
  render() {
    return (
      <section className="PreviewPane">
        <iframe src={this.props.endpoint} />
      </section>
    )
  }
}

export default PreviewPane
