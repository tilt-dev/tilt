function allNodes(selection: Selection): Node[] {
  let result = []
  for (let i = 0; i < selection.rangeCount; i++) {
    let range = selection.getRangeAt(i)
    let start = range?.startContainer
    let end = range?.endContainer
    if (start) {
      result.push(start)
    }
    if (end) {
      result.push(end)
    }
  }
  return result
}

// When you have unselectable elements in your selection,
// Firefox represents this as a series of small ranges.
//
// This code doesn't need to be efficient so we just look at all
// the ranges and get the earliest node in document-space.
function startNode(selection: Selection): Node | null {
  let startNode = selection.focusNode
  allNodes(selection).map(candidate => {
    if (startNode == null) return
    if (
      startNode?.compareDocumentPosition(candidate) &
      Node.DOCUMENT_POSITION_PRECEDING
    ) {
      startNode = candidate
    }
  })
  return startNode
}

// When you have unselectable elements in your selection,
// Firefox represents this as a series of small ranges.
//
// This code doesn't need to be efficient so we just look at all
// the ranges and get the last node in document-space.
function endNode(selection: Selection): Node | null {
  let endNode = selection.focusNode
  allNodes(selection).map(candidate => {
    if (endNode == null) return
    if (
      candidate.compareDocumentPosition(endNode) &
      Node.DOCUMENT_POSITION_PRECEDING
    ) {
      endNode = candidate
    }
  })
  return endNode
}

export default { startNode, endNode }
