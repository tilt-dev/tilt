import React from 'react';
import './Status.css';

function Preview(props) {
    let total = props.resources.filter(resource => {
        return resource.IsTiltFile !== true
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
            <section>{running.length}/{total.length} {maybePluralize(running.length, 'Resource')} Running</section>
            <section>{errors.length} {maybePluralize(errors.length, 'Error')}</section>
            <section>
                <button onClick={props.togglePreview}>Toggle Preview</button>
            </section>
        </footer>
    )
}

function maybePluralize(count, noun, suffix = 's') {
    return `${noun}${count !== 1 ? suffix : ''}`
}

export default Preview;
