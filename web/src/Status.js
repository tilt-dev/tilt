import React from 'react';
import './Status.scss';

function Preview(props) {
    let total = props.resources.filter(resource => {
        return resource.IsTiltfile !== true
    })
    let running = props.resources.filter(resource => {
        return resource.ResourceInfo && resource.ResourceInfo.PodStatus === "Running"
    })
    // More comprehensive error — build, runtime, restarts
    let errors = props.resources.filter(resource => {
        let errorStates = ["Error", "CrashLoopBackOff"]
        let status = resource.ResourceInfo && resource.ResourceInfo.PodStatus
        return errorStates.some(state => status === state)
    })

    return (
        <footer className="status">
            <section>
                <span>{running.length}/{total.length} {maybePluralize(running.length, 'Resource')} Running</span>
                <span> | </span>
                <span>{errors.length} {maybePluralize(errors.length, 'Error')}</span>
            </section>
            {props.showPreview &&
                <button onClick={props.closePreview}>Close Preview</button>
            }
        </footer>
    )
}

function maybePluralize(count, noun, suffix = 's') {
    return `${noun}${count !== 1 ? suffix : ''}`
}

export default Preview;
