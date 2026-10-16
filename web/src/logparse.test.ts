import {
  findKey,
  flattenObject,
  messageKeys,
  parseLogfmt,
  parseStructured,
  StructuredKind,
} from "./logparse"

describe("parseStructured", () => {
  it("parses a JSON line", () => {
    let p = parseStructured(`{"level":"info","msg":"hi","user":{"id":1}}`)
    expect(p).toEqual({
      kind: StructuredKind.JSON,
      prefix: "",
      data: { level: "info", msg: "hi", user: { id: 1 } },
    })
  })

  it("parses a JSON line wrapped in ANSI codes and whitespace", () => {
    let p = parseStructured(`  \x1b[32m{"msg":"green"}\x1b[0m `)
    expect(p.kind).toEqual(StructuredKind.JSON)
  })

  it("parses prefixed JSON (zap console encoder)", () => {
    let p = parseStructured(`INFO\tfinished call\t{"grpc.code": "OK"}`)
    expect(p).toEqual({
      kind: StructuredKind.JSON,
      prefix: "INFO\tfinished call",
      data: { "grpc.code": "OK" },
    })
  })

  it("parses prefixed JSON (container prefix)", () => {
    let p = parseStructured(`[app] {"msg":"started"}`)
    expect(p.kind).toEqual(StructuredKind.JSON)
    expect(p.kind !== StructuredKind.Text && p.prefix).toEqual("[app]")
  })

  it("parses logfmt", () => {
    let p = parseStructured(
      `time=2024-01-01T00:00:00Z level=info msg="request done" status=200`
    )
    expect(p).toEqual({
      kind: StructuredKind.Logfmt,
      prefix: "",
      data: {
        time: "2024-01-01T00:00:00Z",
        level: "info",
        msg: "request done",
        status: "200",
      },
    })
  })

  it("leaves plain text alone", () => {
    expect(parseStructured("Starting server on :8080").kind).toEqual(
      StructuredKind.Text
    )
    expect(parseStructured("").kind).toEqual(StructuredKind.Text)
    expect(parseStructured("retrying, attempt=3").kind).toEqual(
      StructuredKind.Text
    )
  })

  it("rejects malformed JSON and non-object JSON", () => {
    expect(parseStructured(`{"msg": "oops"`).kind).toEqual(StructuredKind.Text)
    expect(parseStructured(`{not json}`).kind).toEqual(StructuredKind.Text)
    expect(parseStructured(`[1, 2, 3]`).kind).toEqual(StructuredKind.Text)
    expect(parseStructured(`function() {}`).kind).toEqual(StructuredKind.Text)
  })
})

describe("parseLogfmt", () => {
  it("handles escaped quotes", () => {
    expect(parseLogfmt(`a="say \\"hi\\"" b=2`)).toEqual({
      a: `say "hi"`,
      b: "2",
    })
  })

  it("handles empty values", () => {
    expect(parseLogfmt(`a= b=2`)).toEqual({ a: "", b: "2" })
  })

  it("rejects unclosed quotes and keys without values", () => {
    expect(parseLogfmt(`a="oops b=2`)).toBeNull()
    expect(parseLogfmt(`a=1 b`)).toBeNull()
  })
})

describe("flattenObject", () => {
  it("flattens nested objects to dotted keys", () => {
    expect(
      flattenObject({ a: 1, b: { c: "x", d: { e: null } }, f: [1, 2], g: {} })
    ).toEqual([
      ["a", 1],
      ["b.c", "x"],
      ["b.d.e", null],
      ["f", [1, 2]],
      ["g", {}],
    ])
  })
})

describe("findKey", () => {
  it("finds the first primitive key", () => {
    expect(findKey({ message: "a", msg: "b" }, messageKeys)).toEqual("msg")
    expect(findKey({ msg: { nested: 1 }, message: "a" }, messageKeys)).toEqual(
      "message"
    )
    expect(findKey({ other: 1 }, messageKeys)).toBeNull()
  })
})
