import React, { PureComponent } from "react"
import "./PreviewPane.scss"

type PreviewProps = {
  endpoint: string
  isExpanded: boolean
}

class PreviewPane extends PureComponent<PreviewProps> {
  render() {
    let classes = `PreviewPane ${this.props.isExpanded &&
      "PreviewPane--expanded"}`

    return (
      <section className={classes}>
        <iframe src={this.props.endpoint} />
      </section>
    )
  }
}

export default PreviewPane
