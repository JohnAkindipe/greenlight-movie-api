package main

import (
	"fmt"
	"greenlight-movie-api/internal/data"
	"net/http"
	"time"
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

    //instantiate a movie type, we'll probably be pulling this data from a database later.
    movie := data.Movie{
        ID: id,
        Title: "Casablanca",
        Runtime: 102,
        Genres: []string{"drama", "romance", "war"},
        Version: 1,
        CreatedAt: time.Now(),
    }

    //wrap the movie data with the string "movie"
    wrappedMovieData := envelope{ "movie": movie }

    err = appPtr.writeJSON(w, http.StatusOK, wrappedMovieData, nil) //marshal the movie data into json and send to the client

    //Respond with an error if we encountered an error marshalling the movie data into valid json
    if err != nil {
        appPtr.logger.Error(err.Error())
        http.Error(w, "We encountered a problem in our server", http.StatusInternalServerError)
    }
}