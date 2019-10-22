import React from "react"
import HeroScreen from "./HeroScreen"

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
  return <HeroScreen message={message} />
}

export default NotFound
