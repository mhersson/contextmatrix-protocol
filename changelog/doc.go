// Package changelog is the reading of a build's merge history: GitLogArgs is
// the git invocation every component's build runs, and Parse turns its output
// into the change list a protocol.BuildInfo carries. contextmatrix and every
// backend build with it, so all four components filter their history by one
// rule.
//
// Scope, deliberately narrow: a pure function from bytes to changes, plus the
// arguments that produce those bytes. The build runs git, not this package, so
// there is no I/O, no logging and no run-time state here.
package changelog
