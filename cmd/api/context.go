package main

import (
	"context"
	"greenlight-movie-api/internal/data"
	"net/http"
)

// Define a custom contextKey type, with the underlying type string.
type contextKey string

// Convert the string "user" to a contextKey type and assign it to the userContextKey
// constant. We'll use this constant as the key for getting and setting user information
// in the request context.
const userContextKey = contextKey("user")

// The contextSetUser() method returns a new copy of the request with the provided
// User struct added to the context. Note that we use our userContextKey constant as the
// key.
func(appPtr *application) contextSetUser(r *http.Request, userPtr *data.User) *http.Request {
	ctx := context.WithValue(context.Background(), userContextKey, userPtr)
	return r.WithContext(ctx)
}

// The contextGetUser() retrieves the User struct from the request context. The only
// time that we'll use this helper is when we logically expect there to be User struct
// value in the context, and if it doesn't exist it will firmly be an 'unexpected' error.
// As we discussed earlier in the book, it's OK to panic in those circumstances.
func(appPtr *application) contextGetUser(r *http.Request) *data.User {
	user := r.Context().Value(userContextKey)
	userPtr, ok := user.(*data.User)
	if !ok {
		panic("user value missing in request context")
	}
	return userPtr
}