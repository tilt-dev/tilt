import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./FacetsPane.scss"

type Resource = Proto.webviewResource

type FacetsProps = {
  resource: Resource
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

class FacetsPane extends PureComponent<FacetsProps> {
  render() {
    let el = (
      <section className="Pane-empty-message">
        <LogoWordmarkSvg />
        <h2>No Facets Found</h2>
      </section>
    )

    let facets = this.renderFacets()
    if (facets.length > 0) {
      el = <ul>{facets}</ul>
    }

    return <section className="FacetsPane">{el}</section>
  }

  renderFacets(): Array<JSX.Element> {
    if (!this.props.resource.facets) {
      return new Array<JSX.Element>()
    }
    return this.props.resource.facets.map(facet => {
      return (
        <li className="FacetsPane-item">
          <header>
            <div className="FacetsPane-headerDiv">
              <h3>{facet.name}</h3>
            </div>
          </header>
          <section>{logToLines(facet.value ?? "")}</section>
        </li>
      )
    })
  }
}

export default FacetsPane
