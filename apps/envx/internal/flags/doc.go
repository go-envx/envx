// Package flags is the cobra binding boundary: it registers schema flag specs as
// flags on cobra commands. BindString/BindBool/BindPersistentString each read a
// schema.FlagSpec (name, shorthand, usage) and bind the matching cobra
// flag, writing parsed values into a caller-owned destination. Housing these
// helpers here keeps the config package (resolution) and the schema package
// (the spec catalog) free of any cobra dependency.
package flags
