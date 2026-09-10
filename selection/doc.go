// Package selection is the deterministic interpretation of the model
// selection wire types: given a SelectionContext and a per-role tier ladder,
// which model a role and tier resolve to, and why. contextmatrix uses it to
// preview picks; contextmatrix-agent uses it to make them, so both see one
// rule.
//
// Scope, deliberately narrow: pure functions from wire types to picks. No
// logging, no I/O, no run-time state, no types from any other module. What a
// caller does with a pick - log it, retry, exclude a model that failed -
// stays with the caller. If a change here needs any of those, it does not
// belong here.
package selection
