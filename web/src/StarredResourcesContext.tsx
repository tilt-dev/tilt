import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from "react"
import { incr } from "./analytics"
import { usePersistentState } from "./LocalStorage"

export type StarredResourcesContext = {
  starredResources: string[]
  starResource: (name: string) => void
  unstarResource: (name: string) => void
  toggleStarResource: (name: string) => void
}

const starredResourceContext = React.createContext<StarredResourcesContext>({
  starredResources: [],
  starResource: (s) => {},
  unstarResource: (s) => {},
  toggleStarResource: (s) => {},
})

export function useStarredResources(): StarredResourcesContext {
  return useContext(starredResourceContext)
}

export function StarredResourceMemoryProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  const [starredResources, setStarredResources] = useState<Array<string>>(
    props.initialValueForTesting || []
  )

  function starResource(name: string) {
    setStarredResources((prevState) => {
      return prevState.includes(name) ? prevState : [...prevState, name]
    })
  }
  1
  function unstarResource(name: string) {
    setStarredResources((prevState) => {
      return prevState.filter((s) => s !== name)
    })
  }

  function toggleStarResource(name: string) {
    if (starredResources.includes(name)) {
      unstarResource(name)
    } else {
      starResource(name)
    }
  }

  return (
    <starredResourceContext.Provider
      value={{
        starredResources: starredResources,
        starResource: starResource,
        unstarResource: unstarResource,
        toggleStarResource: toggleStarResource,
      }}
    >
      {props.children}
    </starredResourceContext.Provider>
  )
}

export function StarredResourcesContextProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  // we renamed pins to stars but kept the local storage name "pinned-resources"
  // so that user's pinned resources show up as starred
  let [starredResources, setStarredResources] = usePersistentState<string[]>(
    "pinned-resources",
    props.initialValueForTesting ?? []
  )

  useEffect(() => {
    incr("ui.web.star", {
      starCount: starredResources.length.toString(),
      action: "load",
    })
    // empty deps because we only want to report the loaded star count once per app load
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function starResource(name: string) {
    setStarredResources((prevState) => {
      const ret = prevState.includes(name) ? prevState : [...prevState, name]
      incr("ui.web.star", {
        starCount: ret.length.toString(),
        action: "star",
      })
      return ret
    })
  }

  function unstarResource(name: string) {
    setStarredResources((prevState) => {
      const ret = prevState.filter((n) => n !== name)
      incr("ui.web.star", {
        starCount: ret.length.toString(),
        action: "unstar",
      })
      return ret
    })
  }

  function toggleStarResource(name: string) {
    if (starredResources.includes(name)) {
      unstarResource(name)
    } else {
      starResource(name)
    }
  }

  return (
    <starredResourceContext.Provider
      value={{
        starredResources: starredResources,
        starResource: starResource,
        unstarResource: unstarResource,
        toggleStarResource: toggleStarResource,
      }}
    >
      {props.children}
    </starredResourceContext.Provider>
  )
}
