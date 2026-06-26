// Package flags is the cobra binding boundary: it registers schema flag specs as
// flags on cobra commands. BindString/BindBool/BindPersistentString each read a
// schema.FlagSpec (name, shorthand, usage) and bind the matching cobra
// flag, writing parsed values into a caller-owned destination. Housing these
// binders here isolates the cobra dependency, keeping the resolution and
// spec-catalog layers framework-free.
package flags
