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

	//return the http handler
	return routerPtr
}