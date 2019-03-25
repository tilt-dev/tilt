import React, { PureComponent } from 'react';
import LogoButton from './LogoButton'
import './Statusbar.scss';

class Statusbar extends PureComponent {
  render() {
    return (<div className="Statusbar">
      <LogoButton onclick={this.props.toggleSidebar} />
      <div>I'm a statusbar!</div>
    </div>)
  }
}

export default Statusbar;
