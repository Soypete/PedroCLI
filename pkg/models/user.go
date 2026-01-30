// Package models contains data structures for the application.
package models

// User represents a user in the system.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
