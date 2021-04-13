import React, { useContext } from "react"
import { matchPath, useHistory } from "react-router-dom"
import { usePathBuilder } from "./PathBuilder"
import { ResourceName } from "./types"

// Resource navigation semantics.
// 1. standardizes navigation
// 2. saves components from having to jump through hoops to get history + pathbuilder
export type ResourceNav = {
  // The currently selected resource.
  selectedResource: string

  // Resource provided from the user URL that didn't exist.
  // Different parts of the UI might display this error differently.
  invalidResource: string

  // Behavior when you click on a link to a resource.
  openResource(name: string): void
}

const resourceNavContext = React.createContext<ResourceNav>({
  selectedResource: "",
  invalidResource: "",
  openResource: (name: string) => {},
})

export function useResourceNav(): ResourceNav {
  return useContext(resourceNavContext)
}

export let ResourceNavContextConsumer = resourceNavContext.Consumer
export let ResourceNavContextProvider = resourceNavContext.Provider

export function ResourceNavProvider(
  props: React.PropsWithChildren<{
    validateResource: (name: string) => boolean
  }>
) {
  let validateResource = (name: string): boolean => {
    // The ALL resource should always validate
    return props.validateResource(name) || name === ResourceName.all
  }

  let history = useHistory()
  let pb = usePathBuilder()
  let selectedResource = ""
  let invalidResource = ""

  let matchResource = matchPath(history.location.pathname, {
    path: pb.path("/r/:name"),
  })
  let candidateResource = decodeURIComponent(
    (matchResource?.params as any)?.name || ""
  )
  if (candidateResource && validateResource(candidateResource)) {
    selectedResource = candidateResource
  } else {
    invalidResource = candidateResource
  }

  let openResource = (name: string) => {
    name = name || ResourceName.all
    let url = pb.encpath`/r/${name}/overview`

    history.push(url)
  }

  let resourceNav = {
    invalidResource: invalidResource,
    selectedResource: selectedResource,
    openResource,
  }
  return (
    <resourceNavContext.Provider value={resourceNav}>
      {props.children}
    </resourceNavContext.Provider>
  )
}
