// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define authentication requirements for mutating endpoints.
//
// Key Components:
//   - requireAuth: Asserts that AuthState is initialized
//
// Dependencies:
//   - None
//
// Error Types:
//   - ErrLoginRequired: returned when AuthState is nil
//
package ytm

func requireAuth(auth *AuthState) error {
	if auth == nil {
		return ErrLoginRequired
	}
	return nil
}
