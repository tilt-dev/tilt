import React, { Component } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import LogStore from "./LogStore"
import "./FacetsPane.scss"

type Resource = Proto.webviewResource

type FacetsProps = {
  resource: Resource
  logStore: LogStore | null
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

class FacetsPane extends Component<FacetsProps> {
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
    let facets = this.props.resource.facets ?? []
    return facets.map((facet, facetIndex) => {
      let logStore = this.props.logStore
      let value = logToLines(facet.value ?? "")
      if (facet.spanId && logStore) {
        let lines = logStore.spanLog([facet.spanId])
        value = lines.map((l, i) => (
          <AnsiLine key={"logLine" + i} line={l.text} />
        ))
      }
      return (
        <li key={"facet" + facetIndex} className="FacetsPane-item">
          <header>
            <div className="FacetsPane-headerDiv">
              <h3>{facet.name}</h3>
            </div>
          </header>
          <section>{value}</section>
        </li>
      )
    })
  }
}

export default FacetsPane
