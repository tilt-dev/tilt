import React from 'react';
import './LoadingScreen.css';

function LoadingScreen(props) {
  let message = props.message || 'Loading…'
  return (
    <div className="LoadingScreen">
      {message}
    </div>
  )
}

export default LoadingScreen;
