package main

import (
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
	"time"
)

/*********************************************************************************************************************/
//POST /v1/movies
//To create a new movie
func (appPtr *application) createMovieHandler (w http.ResponseWriter, r *http.Request) {
    //Create a new movie input struct
    var input data.MovieInput

    //Unmarshal the JSON from request body into the input struct
    //Send a bad request response if any error during unmarshaling
    err := appPtr.readJSON(w, r, &input)
    if err != nil {
        appPtr.badRequestResponse(w, r, err)
        return
    }

    // Copy the input into the movie
    movie := data.Movie{
        Year: input.Year,
        Runtime: input.Runtime,
        Genres: input.Genres,
        Title: input.Title,
    }

    movieValidatorPtr := validator.New()

    // Validate the input from the movie input send a 
    // failedValidationResponse if any errors encountered during validation
    data.ValidateMovie(movieValidatorPtr, &movie)
    if !movieValidatorPtr.Valid() {
        appPtr.failedValidationResponse(w, r, movieValidatorPtr.Errors)
        return
    }
    //Store the movie in our database
   
    fmt.Fprintf(w, "%+v\n", input)
    //Return a response to the user that the movie was created successfully
    appPtr.writeJSON(w, http.StatusOK, envelope{"movie": input}, nil)
    // fmt.Fprintf(w, "Movie created successfully %+v", input)
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

/*********************************************************************************************************************/
/*
NOTES
1 - VALIDATE MOVIE
Firstly, you might be wondering why we’re initializing the Validator instance in our handler and passing it to the 
ValidateMovie() function — rather than initializing it in ValidateMovie() and passing it back as a return value.

This is because as our application gets more complex we will need call multiple validation helpers from our handlers, 
rather than just one like we are above. So initializing the Validator in the handler, and then passing it around, 
gives us more flexibility.

2 - COPYING INPUT TO MOVIE
You might also be wondering why we’re decoding the JSON request into the input struct and then copying the data across, 
rather than just decoding into the Movie struct directly.

The problem with decoding directly into a Movie struct is that a client could provide the keys id and version in their 
JSON request, and the corresponding values would be decoded without any error into the ID and Version fields of the 
Movie struct — even though we don’t want them to be. We could check the necessary fields in the Movie struct after
the event to make sure that they are empty, but that feels a bit hacky, and decoding into an intermediary struct 
(like we are in our handler) is a cleaner, simpler, and more robust approach — albeit a little bit verbose.
*/