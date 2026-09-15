// Package operation provides transport-neutral lifecycle infrastructure for
// long-running work.
//
// Business-domain packages retain validation, execution, and result semantics.
// This package may coordinate work supplied by those domains, but must not
// import a business-domain implementation or interpret a domain result.
package operation
