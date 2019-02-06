import React from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import App from './App';
import AppController from './AppController'

// Assume that the HUD was started with --port=8001
let url = 'ws://localhost:8001/ws/view'
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
