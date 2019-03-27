import React from 'react';
import ReactDOM from 'react-dom';
import './index.scss';
import App from './App';
import LogApp from './LogApp';
import K8sApp from './K8sApp';
import HUD from './HUD';
import NoMatch from './NoMatch';
import { BrowserRouter as Router, Route, Switch } from 'react-router-dom';

let Main = () => {
  return (<Router>
    <div>
      <Switch>
          <Route exact path="/" component={App} />
        {/*
          * New HUD work. HUD will eventually be promoted to the top-level component
          * and we'll delete App & friends
          */}
        <Route path="/hud" component={HUD} />
        <Route exact path="/log" component={LogApp} />
        <Route exact path="/r/:name/log" component={LogApp} />
        <Route exact path="/r/:name/k8s" component={K8sApp} />
        <Route component={NoMatch} />
      </Switch>
    </div>
  </Router>)
}

let app = (<Main />)
let root = document.getElementById('root')
ReactDOM.render(app, root)
