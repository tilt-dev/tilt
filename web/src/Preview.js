import React from 'react';
import './Preview.css';

function Preview(props) {
    // Get Proper Port
    return (
        <iframe className="Preview" title="preview" marginwidth="0" marginheight="0" src="http://localhost:8080/"></iframe>
    )
}

export default Preview;
