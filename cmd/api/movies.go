package main

import (
	"encoding/json"
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
    //Create a new movie struct
    var input struct {
        Title   string   `json:"title"`
        Year    int32    `json:"year"`
        Runtime int32    `json:"runtime"`
        Genres  []string `json:"genres"`
    }
    //Read the request body and Decode the request body into movie struct
    //Send an error response if errors decoding
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        appPtr.errorResponse(w, r, http.StatusBadRequest, err.Error())
    }
    
    //Store the movie in our database
   
    // fmt.Printf("%+v\n", movie)
    fmt.Println(input)
    //Return a response to the user that the movie was created successfully
    fmt.Fprintf(w, "Movie created successfully %+v", input)
}
/*********************************************************************************************************************/
//GET /v1/movies/:id
//To get info about a specific movie
func (appPtr *application) showMovieHandler (w http.ResponseWriter, r *http.Request) {
	//Get the value of the named parameter "id" from the request
    id, err := appPtr.readIDParam(r)
    if err != nil {
        //let the client know we could not find a movie with the provided id parameter
        appPtr.notFoundHandler(w, r)
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
        //log error and send json-formatted error to client
        //log error if unable to format error to json and send empty response with
        //code 500 to client
        appPtr.serverErrorResponse(w, r, err)
    }
}