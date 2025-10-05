package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

const (
	MOVIE_READ = "movies:read"
	MOVIE_WRITE = "movies:write"
)
/*********************************************************************************************************************/
// APPLICATION ROUTER
// Return the router to use for our application
func (appPtr *application) routes() http.Handler {
	// routerptr is an object that satisfies the http.Handler interface by defining a servehttp method
	routerPtr := httprouter.New()

	// register the notFoundResponse helper as the default handler for 
	// requests that could not be matched to any path
	routerPtr.NotFound = http.HandlerFunc(appPtr.notFoundHandler)
	// register the methodNotAllowedResponse helper as the default handler for 
	// requests to a path with methods that the path doesn't allow (e.g a POST
	// request to "healthcheck")
	routerPtr.MethodNotAllowed = http.HandlerFunc(appPtr.methodNotAllowedHandler)
/* 
the handlerfunc will register the function to call for a specific type of request to a particular
endpoint
*/
	// GET "/v1/healthcheck"
	routerPtr.HandlerFunc(http.MethodGet, "/v1/healthcheck", appPtr.healthcheckHandler)

	//POST /v1/movies
	//To create a new movie
	routerPtr.HandlerFunc(http.MethodPost, "/v1/movies", appPtr.requirePermission(MOVIE_WRITE, appPtr.createMovieHandler))
	//GET /v1/movies/:id
	//To get info about a specific movie
	routerPtr.HandlerFunc(http.MethodGet, "/v1/movies/:id", appPtr.requirePermission(MOVIE_READ, appPtr.showMovieHandler))

	//PATCH /v1/movies/:id
	//To update a field in a specific movie
	routerPtr.HandlerFunc(http.MethodPatch, "/v1/movies/:id", appPtr.requirePermission(MOVIE_WRITE, appPtr.updateMovieHandler))

	//PUT /v1/movies/:id
	//To replace an entire movie with a given id in our database
	routerPtr.HandlerFunc(http.MethodPut, "/v1/movies/:id", appPtr.requireActivatedUser(appPtr.replaceMovieHandler))

	//DELETE /v1/movies/:id
	//To delete a specific movie from the db
	routerPtr.HandlerFunc(http.MethodDelete, "/v1/movies/:id", appPtr.requirePermission(MOVIE_WRITE, appPtr.deleteMovieHandler))

	//GET /v1/movies
	//To Get all the movies from the db: Also allows for filtering, sorting, and pagination
	routerPtr.HandlerFunc(http.MethodGet, "/v1/movies", appPtr.requirePermission(MOVIE_READ, appPtr.showAllMoviesHandler))

	//USERS ENDPOINT
	//POST /v1/users
	//To register(create) a new user
	routerPtr.HandlerFunc(http.MethodPost, "/v1/users", appPtr.registerUserHandler)

	//PUT /v1/users/activated
	//To activate a specific user
	routerPtr.HandlerFunc(http.MethodPut, "/v1/users/activated", appPtr.activateUserHandler)

	//TOKENS
	//STANDALONE ACTIVATION ENDPOINT
	//POST /v1/tokens/activation
	//Specifically to generate a new activation token such as if a user doesn't initially activate their account 
	//before token expiry or they never receive the welcome email containing the token for some reason.
	routerPtr.HandlerFunc(http.MethodPost, "/v1/tokens/activation", appPtr.createActivationTokenHandler)
	//POST /v1/tokens/authentication
	//Authentication Token Generation
	//Allow a client to exchange their credentials (email address and password) for a stateful authentication token.
	routerPtr.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", appPtr.createAuthenticationTokenHandler)
	//POST /v1/tokens/jwt-authentication
	//Generates a JWT Token for Authentication
	routerPtr.HandlerFunc(http.MethodPost, "/v1/tokens/jwt-authentication", appPtr.createJWTAuthenticationTokenHandler)
	//return the http handler
	// recoverPanic -> rateLimit -> authenticate -> appRouter
	return appPtr.recoverPanic(appPtr.rateLimit(appPtr.authenticate(routerPtr)))
}