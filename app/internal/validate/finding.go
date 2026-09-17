package validate

import "github.com/go-envx/envx/app/internal/status"

// Severity ranks a finding. It mirrors the envmerge severities so a resolution
// outcome maps onto a finding without translation loss.
type Severity string

const (
	// SeverityWarning marks a non-fatal finding, such as an unavailable key or an
	// orphaned store value.
	SeverityWarning Severity = "warning"
	// SeverityError marks a failing finding, such as a dangling reference or a
	// plaintext store value.
	SeverityError Severity = "error"
)

// Finding is one validation problem. Reference findings carry the project and
// environment they were observed in; store-level findings leave both empty and
// identify the entry through Key. No field ever carries secret or private-key
// material.
type Finding struct {
	// Severity ranks the finding.
	Severity Severity
	// Project is the project a reference finding was observed in; empty for a
	// store-level finding.
	Project string
	// Environment is the environment a reference finding was observed in; empty
	// for a store-level finding.
	Environment string
	// Key identifies the subject: the env-var key for a reference finding, or the
	// "group/key" (or group, for a keypair) for a store-level finding.
	Key string
	// Code is the stable, machine-classifiable status identifier.
	Code string
	// Message is a human-readable description free of secret material.
	Message string
}

// Params controls how validation grades its outcome.
type Params struct {
	// Strict fails the run on warnings as well as errors. Errors always fail.
	Strict bool
	// Severity overrides the default reporting level per status code, keyed by
	// canonical code. A nil map leaves every code at its default; a code mapped to
	// status.Off suppresses its findings entirely.
	Severity map[string]status.Severity
	// Selected lists the checks to run, keyed by canonical status code. A nil or
	// empty map runs every check; a non-empty map runs only the selected checks and
	// skips the cost of the rest, so a hook can run only the offline store checks.
	Selected map[string]bool
}

// level resolves the reporting level for a status code: the configured override
// if present, else the code's built-in default. An unknown code defaults to
// error so a new, unmapped code fails loudly rather than passing silently.
func (p Params) level(code string) status.Severity {
	if severity, ok := p.Severity[code]; ok {
		return severity
	}
	if severity, ok := status.DefaultSeverity(code); ok {
		return severity
	}
	return status.Error
}

// Report is the aggregate validation outcome: every finding plus the counts and
// the pass/fail verdict a command maps to an exit code.
type Report struct {
	// Findings lists every problem observed, in the order they were added.
	Findings []Finding
	// Errors counts findings at error severity.
	Errors int
	// Warnings counts findings at warning severity.
	Warnings int
	// Failed reports whether the run should exit non-zero: any error, or any
	// warning under strict.
	Failed bool
}

// record grades a finding by its code and adds it unless the code is configured
// off. The severity is assigned from the code's resolved level, not from the
// caller, so one code carries one severity across every check. A finding whose
// code resolves to status.Off is dropped entirely.
func (r *Report) record(params Params, f Finding) {
	switch params.level(f.Code) {
	case status.Off:
		return
	case status.Warn:
		f.Severity = SeverityWarning
	default:
		f.Severity = SeverityError
	}
	r.add(f)
}

// add appends an already-graded finding and updates the severity tallies.
func (r *Report) add(f Finding) {
	r.Findings = append(r.Findings, f)
	switch f.Severity {
	case SeverityError:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	}
}

// grade sets Failed from the accumulated counts under the strict policy: an error
// always fails, and a warning fails only under strict.
func (r *Report) grade(strict bool) {
	r.Failed = r.Errors > 0 || (strict && r.Warnings > 0)
}
