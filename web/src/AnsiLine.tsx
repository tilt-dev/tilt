import React from "react"
import Ansi from "ansi-to-react"

type AnsiLineProps = {
  line: string
  className?: string
}

let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <Ansi linkify={false} useClasses={true} className={props.className}>
      {props.line}
    </Ansi>
  )
})

export default AnsiLine
