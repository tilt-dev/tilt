import React from 'react';
import ReactDOM from 'react-dom';
import './index.scss';
import App from './App';
import LogApp from './LogApp';
import LoadingScreen from './LoadingScreen';
import { BrowserRouter as Router, Route, Switch, withRouter } from 'react-router-dom';

let NoMatch = ({location}) => {
  let message = (<div>No match for <code>{location.pathname}</code></div>)
  return <LoadingScreen message={message} />
};

let Main = () => {
  return (<Router>
    <div>
      <Switch>
        <Route exact path="/" component={App} />
        <Route exact path="/log" component={LogApp} />
        <Route exact path="/r/:name/log" component={LogApp} />
        <Route component={NoMatch} />
      </Switch>
    </div>
  </Router>)
}

let app = (<Main />)
let root = document.getElementById('root')
ReactDOM.render(app, root)
