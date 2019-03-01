import React from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import App from './App';
import AppController from './AppController'

let url = `ws://${window.location.host}/ws/view`
let renderAsync = new Promise((resolve, reject) => {
  let app = (<App />)
  let root = document.getElementById('root')
  ReactDOM.render(app, root, function() {
    resolve(this)
  })
})

renderAsync.then((component) => {
  new AppController(url, component)
})
