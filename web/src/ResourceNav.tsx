import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react"
import { matchPath, useNavigate, useLocation } from "react-router-dom"
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
  let validateResource = useCallback(
    (name: string): boolean => {
      // The ALL resource should always validate
      return props.validateResource(name) || name === ResourceName.all
    },
    [props.validateResource]
  )

  const navigate = useNavigate()
  let location = useLocation()
  let pb = usePathBuilder()
  let selectedResource = ""
  let [filterByResource, setFilterByResource] = useState(
    {} as { [key: string]: string }
  )
  let invalidResource = ""

  let matchResource =
    matchPath({ path: pb.path("/r/:name") }, location.pathname) ||
    matchPath({ path: pb.path("/r/:name/*") }, location.pathname)
  let candidateResource = decodeURIComponent(
    (matchResource?.params as any)?.name || ""
  )
  if (candidateResource && validateResource(candidateResource)) {
    selectedResource = candidateResource
  } else {
    invalidResource = candidateResource
  }

  let search = location.search

  useEffect(() => {
    let existing = filterByResource[selectedResource] || ""
    if (existing != search) {
      let obj = {} as { [key: string]: string }
      Object.assign(obj, filterByResource)
      obj[selectedResource] = search
      setFilterByResource(obj)
    }
  }, [selectedResource, search])

  let openResource = useCallback(
    (name: string) => {
      name = name || ResourceName.all
      let url = pb.encpath`/r/${name}/overview`

      // We deliberately make search terms stick to a resource.
      //
      // So if you add a log filter to resource A, navigate to B,
      // then come back to A, we preserve the filter on A.
      //
      // We're not sure if this is the right behavior, and do not
      // store it in any sort of persistent store.
      let storedFilter = filterByResource[name] || ""
      navigate(url + storedFilter)
    },
    [navigate, filterByResource]
  )

  let resourceNav = useMemo(() => {
    return {
      invalidResource: invalidResource,
      selectedResource: selectedResource,
      openResource,
    }
  }, [invalidResource, selectedResource, openResource])

  return (
    <resourceNavContext.Provider value={resourceNav}>
      {props.children}
    </resourceNavContext.Provider>
  )
}
