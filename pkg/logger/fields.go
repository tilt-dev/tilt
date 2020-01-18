package logger

// Allow arbitrary fields on a log.
// Inspired by the logrus.Fields API
// https://github.com/sirupsen/logrus
type Fields map[string]string

const FieldNameProgressID = "progressID"
