import React, { PureComponent } from "react"
import styled from "styled-components"
import { Link } from "react-router-dom"
import { LogTrace, LogTraceNav, ResourceView } from "./types"
import * as s from "./style-helpers"

type NavProps = {
  logUrl: string
  traceNav: LogTraceNav | null
  alertsUrl: string
  facetsUrl: string | null
  resourceView: ResourceView
  numberOfAlerts: number
}

let Root = styled.nav`
  display: flex;
  flex-direction: column;
`

let NavList = styled.ul`
  display: flex;
  list-style: none;
  height: ${s.Height.secondaryNav}px;
`

let NavListLower = styled.div`
  display: flex;
  background-color: ${s.Color.grayDark};
  border-top: 2px solid ${s.Color.gray};
  border-bottom: 2px solid ${s.Color.gray};
  height: ${s.Height.secondaryNavLower}px;
  margin-top: ${s.Height.secondaryNavOverlap}px;
  font-size: ${s.FontSize.smallest};
  font-family: ${s.Font.sansSerif};
  align-items: stretch;
`

let NavListLowerItem = styled.div`
  border-right: 2px solid ${s.Color.gray};
  padding: 0 ${s.SizeUnit(0.5)};
  display: flex;
  justify-content: center;
  align-items: center;
  box-sizing: border-box;
  text-align: center;

  &:first-child {
    width: ${s.Width.secondaryNavItem}px;
  }
`

let NavListLowerLink = styled(Link)`
  border-right: 2px solid ${s.Color.gray};
  padding: 0 ${s.SizeUnit(0.5)};
  display: flex;
  align-items: center;

  background-color: transparent;
  color: ${s.Color.grayLight};
  box-sizing: border-box;
  transition: color, border-color;
  transition-duration: ${s.AnimDuration.default};
  text-decoration: none;
  cursor: pointer;

  &:hover {
    background-color: ${s.Color.gray};
    color: ${s.Color.blue};
  }

  &.is-disabled {
    color: ${s.Color.gray};
    pointer-events: none;
  }
`

let NavListItem = styled.li`
  display: flex;
  align-items: stretch;
`

let NavLink = styled(Link)`
  display: flex;
  justify-content: center;
  align-items: center;
  font-family: ${s.Font.sansSerif};
  font-size: ${s.FontSize.small};
  text-decoration: none;
  width: ${s.Width.secondaryNavItem}px;
  text-align: center;
  border-top-left-radius: ${s.SizeUnit(0.2)};
  border-top-right-radius: ${s.SizeUnit(0.2)};
  color: ${s.Color.grayLight};
  transition: color ${s.AnimDuration.default} ease;

  &.isSelected {
    color: ${s.Color.white};
    background-color: ${s.Color.grayDark};
    box-shadow: -3px -3px 2px 1px ${s.Color.grayDarkest};
  }

  &:hover {
    color: ${s.Color.blue};
  }
`

let Badge = styled.div`
  font-family: ${s.Font.sansSerif};
  font-size: ${s.FontSize.smallest};
  background-color: ${s.Color.white};
  color: ${s.Color.grayDarkest};
  width: ${s.Width.badge}px;
  height: ${s.Width.badge}px;
  border-radius: ${s.Width.badge}px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: ${s.SizeUnit(0.3)};
`

class SecondaryNav extends PureComponent<NavProps> {
  renderSecondLevelNav() {
    let traceIsSelected = this.props.resourceView === ResourceView.Trace
    let traceNav = this.props.traceNav
    if (!traceIsSelected || !traceNav) {
      return null
    }

    let current = traceNav.current
    let prev = traceNav.prev
    let next = traceNav.next

    let secondLevelItems = []
    secondLevelItems.push(
      <NavListLowerItem key={"trace-label"}>For Update</NavListLowerItem>
    )

    secondLevelItems.push(
      <NavListLowerLink
        key={"trace-prev"}
        to={prev?.url || ""}
        title="Previous"
        className={prev ? "" : "is-disabled"}
      >
        ◄
      </NavListLowerLink>
    )

    let summary = `${current.index + 1} / ${traceNav.count}`
    secondLevelItems.push(
      <NavListLowerItem key={"trace-summary"}>{summary}</NavListLowerItem>
    )

    secondLevelItems.push(
      <NavListLowerLink
        key={"trace-next"}
        to={next?.url || ""}
        title="Next"
        className={next ? "" : "is-disabled"}
      >
        ►
      </NavListLowerLink>
    )

    return <NavListLower>{secondLevelItems}</NavListLower>
  }

  render() {
    let traceIsSelected = this.props.resourceView === ResourceView.Trace
    let logIsSelected =
      this.props.resourceView === ResourceView.Log || traceIsSelected
    let alertsIsSelected = this.props.resourceView === ResourceView.Alerts
    let facetsIsSelected = this.props.resourceView === ResourceView.Facets

    let secondLevelNav = this.renderSecondLevelNav()

    let facetItem = null
    if (this.props.facetsUrl) {
      facetItem = (
        <NavListItem>
          <NavLink
            className={`tabLink ${facetsIsSelected ? "isSelected" : ""}`}
            to={this.props.facetsUrl}
          >
            Facets
          </NavLink>
        </NavListItem>
      )
    }

    // The number of alerts should be for the selected resource
    return (
      <Root className="secondaryNav">
        <NavList>
          <NavListItem>
            <NavLink
              className={logIsSelected ? "isSelected" : ""}
              to={this.props.logUrl}
            >
              Logs
            </NavLink>
          </NavListItem>
          <NavListItem>
            <NavLink
              className={`secondaryNavLink--alerts ${
                alertsIsSelected ? "isSelected" : ""
              }`}
              to={this.props.alertsUrl}
            >
              Alerts
              {this.props.numberOfAlerts > 0 && (
                <Badge>{this.props.numberOfAlerts}</Badge>
              )}
            </NavLink>
          </NavListItem>
          {facetItem}
        </NavList>
        {secondLevelNav}
      </Root>
    )
  }
}

export default SecondaryNav
