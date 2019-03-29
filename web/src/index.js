import React from 'react';
import ReactDOM from 'react-dom';
import './index.scss';
import HUD from './HUD';
import NoMatch from './NoMatch';
import { BrowserRouter as Router, Route, Switch } from 'react-router-dom';

let Main = () => {
  return (<Router>
    <div>
      <Switch>
        <Route path="/hud" component={HUD} />
        <Route component={NoMatch} />
      </Switch>
    </div>
  </Router>)
}

let app = (<Main />)
let root = document.getElementById('root')
ReactDOM.render(app, root)
