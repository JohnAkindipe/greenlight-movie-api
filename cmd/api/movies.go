package main

import (
	"fmt"
	"net/http"
)

/*********************************************************************************************************************/
//POST /v1/movies
//To create a new movie
func (appPtr *application) createMovieHandler (w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Create a movie")
}
/*********************************************************************************************************************/
//GET /v1/movies/:id
//To get info about a specific movie
func (appPtr *application) showMovieHandler (w http.ResponseWriter, r *http.Request) {
	//Get the value of the named parameter "id" from the request
    id, err := appPtr.readIDParam(r)
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // Otherwise, interpolate the movie ID in a placeholder response.
    fmt.Fprintf(w, "show the details of movie %d\n", id)
}