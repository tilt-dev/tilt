import React, { PureComponent } from 'react';
import { isZeroTime } from './time';
import { Link } from 'react-router-dom';
import './Sidebar.scss';

class SidebarItem {
  name: string;
  status: string;
  
  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name

    let runtimeStatus = res.RuntimeStatus
    let currentBuild = res.CurrentBuild
    let hasCurrentBuild = Boolean(currentBuild && !isZeroTime(currentBuild.StartTime))
    let hasPendingBuild = !isZeroTime(res.PendingBuildSince)

    this.status = runtimeStatus
    if (hasCurrentBuild || hasPendingBuild) {
      this.status = 'pending'
    }
  }
}

type SidebarProps = {
  isOpen: boolean,
  items: SidebarItem[],
}

class Sidebar extends PureComponent<SidebarProps> {
  render() {
    let classes = ['Sidebar']
    if (this.props.isOpen) {
      classes.push('is-open')
    }

    let listItems = this.props.items.map((item) => {
      let link = `/hud/r/${item.name}`
      return (<li key={item.name} className="resLink">
        <Link to={link}>{item.name}</Link>
      </li>)        
    })
    
    return (<nav className={classes.join(' ')}>
      <h2 className="Sidebar-header">Resources:</h2>
      <ul>
        <li className="resLink resLink--all">
          <Link to="/hud">All</Link>
        </li>
        {listItems}
      </ul>    
    </nav>)
  }
}

export default Sidebar;

export {SidebarItem};
