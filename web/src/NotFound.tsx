import React from "react"
import HeroScreen from "./HeroScreen"

type location = {
  location: {
    pathname: string
  }
}
let NotFound = ({ location }: location) => {
  return (
    <HeroScreen>
      <div>
        No resource found at <code>{location.pathname}</code>
      </div>
    </HeroScreen>
  )
}

export default NotFound
