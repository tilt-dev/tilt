import React from "react"
import Ansi from "ansi-to-react"

type AnsiLineProps = {
  line: string
}

let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <Ansi linkify={false} useClasses={true}>
      {props.line}
    </Ansi>
  )
})

export default AnsiLine
