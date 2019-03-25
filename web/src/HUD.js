import React, { Component } from 'react';
import AppController from './AppController';
import NoMatch from './NoMatch';
import LoadingScreen from './LoadingScreen';
import Sidebar from './Sidebar';
import Statusbar from './Statusbar';
import ResourceViewPane from './ResourceViewPane';
import LogViewPane from './LogViewPane';
import K8sViewPane from './K8sViewPane';
import PreviewPane from './PreviewPane';
import { BrowserRouter as Router, Route, Switch } from 'react-router-dom';
import './HUD.scss';

// The Main HUD view, as specified in
// https://docs.google.com/document/d/1VNIGfpC4fMfkscboW0bjYYFJl07um_1tsFrbN-Fu3FI/edit#heading=h.l8mmnclsuxl1
class HUD extends Component {
  constructor(props) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      Message: '',
      View: {Resources: []}
    }
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  setAppState(state) {
    this.setState(state)
  }

  render() {
    let view = this.state.View
    let message = this.state.Message
    if (!view || !view.Resources || !view.Resources.length) {
      return (<LoadingScreen message={message} />)
    }
    return (
      <Router>
        <div className="HUD">
        <Sidebar />
        <Statusbar />
        <Switch>
          <Route exact path="/hud" render={() => <ResourceViewPane />} />
          <Route exact path="/hud/log" render={() => <LogViewPane />} />
          <Route exact path="/hud/r/:name/log" render={() => <LogViewPane />} />
          <Route exact path="/hud/r/:name/k8s"  render={() => <K8sViewPane />}  />
          <Route exact path="/hud/r/:name/preview" render={() => <PreviewPane />} />
          <Route component={NoMatch} />
        </Switch>
        </div>
      </Router>
    );
  }
}

export default HUD;
