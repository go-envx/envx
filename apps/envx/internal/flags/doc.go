// Package flags is the cobra binding boundary: it registers settings specs as
// flags on cobra commands. BindString/BindBool/BindPersistentString each read a
// settings.Spec (name, shorthand, default, usage) and bind the matching cobra
// flag, writing parsed values into a caller-owned destination. Housing these
// helpers here keeps the config package (resolution) and the settings package
// (the spec catalog) free of any cobra dependency.
package flags
