import { Location } from "history"
import {
  EMPTY_FILTER_TERM,
  filterSetFromLocation,
  parseTermInput,
  TermState,
} from "./logfilters"

enum TestStrings {
  Basic = "abc",
  BuildCommand = 'Step 1 - 0.00s (Running command: [sh -c services="red" ./generate-start.sh] (in "/Users/lizz/Documents/Repos/pixeltilt/full"))',
  BuildError404 = "Build Failed: ImageBuild: failed to compute cache key: failed to walk /var/lib/docker/tmp/buildkit-mount767282166/red: lstat /var/lib/docker/tmp/buildkit-mount767282166/red: no such file or directory",
  BuildErrorInFile = "ERROR IN: [5/5] ADD storage/main ./",
  BuildInfoLine = "[1/5] FROM docker.io/library/alpine@sha256:69e70a79f2d41ab5d637de98c1e0b055206ba40a8145e7bddb55ccc04e13cf8f",
  SyntaxError = "→ render/start.go:9:33: syntax error: unexpected N, expecting comma or )",
}

describe("Log filters", () => {
  describe("state generation", () => {
    describe("for term filter", () => {
      it("gets set with an empty state if no term is present", () => {
        const emptyTermLocation = { search: "term=" } as Location
        expect(filterSetFromLocation(emptyTermLocation).term).toEqual(
          EMPTY_FILTER_TERM
        )

        const noTermLocation = { search: "" } as Location
        expect(filterSetFromLocation(noTermLocation).term).toEqual(
          EMPTY_FILTER_TERM
        )
      })

      it("gets set with a parsed state if a valid term is present", () => {
        const textLocation = { search: "term=docker+build" } as Location
        const textParsedTerm = filterSetFromLocation(textLocation).term
        expect(textParsedTerm.state).toEqual(TermState.Parsed)
        expect(textParsedTerm.input).toEqual("docker build")
        expect(textParsedTerm.hasOwnProperty("regexp")).toBe(true)
        expect(textParsedTerm.hasOwnProperty("error")).toBe(false)

        const regexpLocation = { search: "term=%2Fdocker%2F" } as Location
        const regexpParsedTerm = filterSetFromLocation(regexpLocation).term
        expect(regexpParsedTerm.state).toEqual(TermState.Parsed)
        expect(regexpParsedTerm.input).toEqual("/docker/")
        expect(regexpParsedTerm.hasOwnProperty("regexp")).toBe(true)
        expect(regexpParsedTerm.hasOwnProperty("error")).toBe(false)
      })

      it("gets set with an error state if an invalid input is present", () => {
        const location = { search: "term=%2Fdock(er%3F%2F" } as Location
        const parsedTerm = filterSetFromLocation(location).term
        expect(parsedTerm.state).toEqual(TermState.Error)
        expect(parsedTerm.input).toEqual("/dock(er?/")
        expect(parsedTerm.hasOwnProperty("regexp")).toBe(false)
        expect(parsedTerm.hasOwnProperty("error")).toBe(true)
      })
    })

    describe("term parsing", () => {
      describe("for string literals", () => {
        it("matches on the expected text", () => {
          expect(parseTermInput("abc").test(TestStrings.Basic)).toBe(true)
          expect(parseTermInput("SERVICE").test(TestStrings.BuildCommand)).toBe(
            true
          )
          expect(
            parseTermInput("random phrase").test(TestStrings.BuildInfoLine)
          ).toBe(false)
          expect(parseTermInput("ab5d63").test(TestStrings.BuildInfoLine)).toBe(
            true
          )
          expect(parseTermInput("error").test(TestStrings.SyntaxError)).toBe(
            true
          )
          expect(parseTermInput("mount").test(TestStrings.BuildError404)).toBe(
            true
          )
          expect(parseTermInput("mount ").test(TestStrings.BuildError404)).toBe(
            false
          )
        })

        it("is not stateful", () => {
          let parsed = parseTermInput("line 1")
          expect(parsed.test("line 123")).toBe(true)
          expect(parsed.test("line 124")).toBe(true)
          expect(parsed.test("line 125")).toBe(true)
          expect(parsed.test("line 126")).toBe(true)
        })

        it("escapes any RegExp-specific characters", () => {
          expect(parseTermInput("ab?c").test(TestStrings.Basic)).toBe(false)
          expect(parseTermInput('"red"').test(TestStrings.BuildCommand)).toBe(
            true
          )
          expect(
            parseTermInput("generate-start.sh").test(TestStrings.BuildCommand)
          ).toBe(true)
          expect(parseTermInput("w").test(TestStrings.BuildInfoLine)).toBe(
            false
          )
          expect(
            parseTermInput("comma or )").test(TestStrings.SyntaxError)
          ).toBe(true)
          expect(
            parseTermInput("ERROR.+").test(TestStrings.BuildErrorInFile)
          ).toBe(false)
          expect(parseTermInput("[1/5]").test(TestStrings.BuildInfoLine)).toBe(
            true
          )
        })
      })

      describe("for regular expressions", () => {
        it("only parses strings surrounded by forward slashes as regexp", () => {
          expect(
            parseTermInput("/docker/").test(TestStrings.BuildInfoLine)
          ).toBe(true)
          expect(
            parseTermInput("/docker").test(TestStrings.BuildInfoLine)
          ).toBe(false)
        })

        it("matches on the expected text", () => {
          expect(parseTermInput("/ab?c/").test(TestStrings.Basic)).toBe(true)
          expect(
            parseTermInput("/error.+main/").test(TestStrings.BuildErrorInFile)
          ).toBe(true)
          expect(parseTermInput("/d+/").test(TestStrings.Basic)).toBe(false)
          expect(parseTermInput("/d+/").test(TestStrings.BuildInfoLine)).toBe(
            true
          )
          expect(
            parseTermInput("/failed:/").test(TestStrings.BuildError404)
          ).toBe(true)
        })

        it("throws an error when input text is invalid regex", () => {
          expect(() =>
            parseTermInput("/(missing)? parenthesis)/")
          ).toThrowError(
            "Invalid regular expression: /(missing)? parenthesis)/: Unmatched ')'"
          )
        })
      })
    })
  })
})
