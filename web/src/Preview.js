import React, { Component } from 'react';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import './Preview.css';

function Preview(props) {
    // Get Proper Port
    let children = props.resources.map((resource) => {
        return <Status key={resource.Name} resource={resource} />
    })

    return (
        <iframe className="Preview" src="http://localhost:8080/"></iframe>
    )
}


class Status extends Component {
    render() {
        let resource = this.props.resource

        return (
            <li>{resource.Name}</li>
        );
    }
}

export default Preview;
