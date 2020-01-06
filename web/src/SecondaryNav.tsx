import React, { PureComponent } from "react"
import styled from "styled-components"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import * as s from "./style-helpers"

type NavProps = {
  logUrl: string
  alertsUrl: string
  facetsUrl: string | null
  resourceView: ResourceView
  numberOfAlerts: number
}

let Root = styled.nav`
  height: ${s.Height.secondaryNav}px;
  display: flex;
  align-items: stretch;
`

let NavList = styled.ul`
  display: flex;
  list-style: none;
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
  render() {
    let logIsSelected = this.props.resourceView === ResourceView.Log
    let alertsIsSelected = this.props.resourceView === ResourceView.Alerts
    let facetsIsSelected = this.props.resourceView === ResourceView.Facets

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
          {this.props.facetsUrl === null ? null : (
            <NavListItem>
              <NavLink
                className={`tabLink ${facetsIsSelected ? "isSelected" : ""}`}
                to={this.props.facetsUrl}
              >
                Facets
              </NavLink>
            </NavListItem>
          )}
        </NavList>
      </Root>
    )
  }
}

export default SecondaryNav
