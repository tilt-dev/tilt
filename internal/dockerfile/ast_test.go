package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintBasicAST(t *testing.T) {
	assertPrintSame(t, `

FROM golang:10
RUN echo hi


ADD . .

RUN echo bye
`)
}

func TestPrintHeredoc(t *testing.T) {
	assertPrintSame(t, `
FROM golang:10

COPY <<FILE1 <<FILE2 /dest
content 1
FILE1
content 2
content 2 line 2
FILE2

RUN <<EOF
echo hello
EOF
`)
}

func TestPrintASTRemovesComments(t *testing.T) {
	assertPrint(t, `
# comment
FROM golang:10
RUN echo bye
`, `

FROM golang:10
RUN echo bye
`)
}

func TestPrintCmd(t *testing.T) {
	assertPrintSame(t, `
FROM golang:10
CMD echo bye
`)
}

func TestPrintCmdJSON(t *testing.T) {
	assertPrintSame(t, `
FROM golang:10
CMD ["sh", "-c", "echo bye"]
`)
}

func TestPrintLabel(t *testing.T) {
	// Examples taken from
	// https://docs.docker.com/engine/reference/builder/#label
	assertPrint(t, `
LABEL key1=val1 key2=val2
LABEL "com.example.vendor"="ACME Incorporated"
LABEL com.example.label-with-value="foo"
LABEL version="1.0"
LABEL description="This text illustrates \
that label-values can span multiple lines."
`, `
LABEL key1=val1 key2=val2
LABEL "com.example.vendor"="ACME Incorporated"
LABEL com.example.label-with-value="foo"
LABEL version="1.0"
LABEL description="This text illustrates that label-values can span multiple lines."
`)
}

func TestPrintCopyFlags(t *testing.T) {
	assertPrintSame(t, `
FROM golang:10
COPY --from=gcr.io/windmill/image-a /src /src
RUN echo bye
`)
}

func TestPrintCopyFlagsLabel(t *testing.T) {
	assertPrintSame(t, `
FROM golang:10
COPY --from=gcr.io/windmill/image-a:latest /src /src
RUN echo bye
`)
}

func TestPrintSyntaxDirective(t *testing.T) {
	assertPrintSame(t, `# syntax = foobarbaz

FROM golang:10
RUN echo hi
RUN echo bye
`)
}

func TestMultipleDirectivesOrderDeterministic(t *testing.T) {
	orig := `# syntax = dockerfile:1
# escape = \
# unknown = foo

FROM golang:10
`
	// known directives should be preserved
	// unknown directives should be dropped
	expected := `# syntax = dockerfile:1
# escape = \


FROM golang:10
`

	assertPrint(t, orig, expected)
}

func TestPrintArgAndEnv(t *testing.T) {
	assertPrintSame(t, `

FROM busybox
ARG APP_NAME=TheAppName

ENV APP_NAME=$APP_NAME
`)
}

func TestPrintMultipleEnv(t *testing.T) {
	assertPrintSame(t, `
FROM busybox
ENV VAR_1=foo VAR_2=bar
`)
}

func TestPrintReformatEnv(t *testing.T) {
	assertPrint(t, `
FROM busybox
ENV APP_NAME TheAppName
`, `
FROM busybox
ENV APP_NAME=TheAppName
`)
}

// Convert the dockerfile into an AST, print it, and then
// assert that the result is the same as the original.
func assertPrintSame(t *testing.T, original string) {
	assertPrint(t, original, original)
}

// Convert the dockerfile into an AST, print it, and then
// assert that the result is as expected.
func assertPrint(t *testing.T, original, expected string) {
	df := Dockerfile(original)
	ast, err := ParseAST(df)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := ast.Print()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expected, string(actual))
}
