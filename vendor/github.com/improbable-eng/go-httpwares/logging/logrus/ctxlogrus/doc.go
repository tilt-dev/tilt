// ctxlogrus allows you to store or extract a logrus logger from the context.
//
// When a logger is added to the context (ToContext) its fields are transfered over and will be present when the logger
// is retrieved later using Export.
// If a logger is already present on the context you will override it with the new logger passed and override any fields
// present.
//
// Additional fields can be added to the logger on the context by with the AddFields method.
package ctxlogrus
