import React from "react"
import LoadingScreen from "./LoadingScreen"

type location = {
  location: {
    pathname: string
  }
}
let NotFound = ({ location }: location) => {
  let message = (
    <div>
      No resource found at <code>{location.pathname}</code>
    </div>
  )
  return <LoadingScreen message={message} />
}

export default NotFound
