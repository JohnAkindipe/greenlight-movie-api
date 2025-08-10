package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
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
	routerPtr.HandlerFunc(http.MethodPost, "/v1/movies", appPtr.createMovieHandler)

	//GET /v1/movies/:id
	//To get info about a specific movie
	routerPtr.HandlerFunc(http.MethodGet, "/v1/movies/:id", appPtr.showMovieHandler)

	//PATCH /v1/movies/:id
	//To update a field in a specific movie
	routerPtr.HandlerFunc(http.MethodPatch, "/v1/movies/:id", appPtr.updateMovieHandler)

	//PUT /v1/movies/:id
	//To replace an entire movie with a given id in our database
	routerPtr.HandlerFunc(http.MethodPut, "/v1/movies/:id", appPtr.replaceMovieHandler)

	//DELETE /v1/movies/:id
	//To delete a specific movie from the db
	routerPtr.HandlerFunc(http.MethodDelete, "/v1/movies/:id", appPtr.deleteMovieHandler)

	//GET /v1/movies
	//To Get all the movies from the db: Also allows for filtering, sorting, and pagination
	routerPtr.HandlerFunc(http.MethodGet, "/v1/movies", appPtr.showAllMoviesHandler)

	//USERS ENDPOINT
	//POST /v1/users
	//To register(create) a new user
	routerPtr.HandlerFunc(http.MethodPost, "/v1/users", appPtr.registerUserHandler)

	//PUT /v1/users/activated
	//To activate a specific user
	routerPtr.HandlerFunc(http.MethodPut, "/v1/users/activated", appPtr.activateUserHandler)
	
	//return the http handler
	// recoverPanic -> rateLimit -> appRouter
	return appPtr.recoverPanic(appPtr.rateLimit(routerPtr))
}