import React from "react"
import HeroScreen from "./HeroScreen"

type location = {
  location: {
    pathname: string
  }
}

let NoMatch = ({ location }: location) => {
  let message = (
    <div>
      No match for <code>{location.pathname}</code>
    </div>
  )
  return <HeroScreen message={message} />
}

export default NoMatch
