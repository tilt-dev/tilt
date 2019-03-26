import React, { PureComponent } from 'react';
import './Sidebar.scss';

class Sidebar extends PureComponent {
  render() {
    let classes = ['Sidebar']
    if (this.props.isOpen) {
      classes.push('is-open')
    }
    return (<div className={classes.join(' ')}>I'm a sidebar!</div>)
  }
}

export default Sidebar;
