package logger

// Allow arbitrary fields on a log.
// Inspired by the logrus.Fields API
// https://github.com/sirupsen/logrus
type Fields map[string]string

const FieldNameProgressID = "progressID"
const FieldNameBuildEvent = "buildEvent"

// Most progress lines are optional. For example, if a bunch
// of little upload updates come in, it's ok to skip some.
//
// progressMustPrint="1" indicates that this line must appear in the
// output - e.g., a line that communicates that the upload finished.
const FieldNameProgressMustPrint = "progressMustPrint"
