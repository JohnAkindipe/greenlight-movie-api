package main

import (
	"errors"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
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

    // Validate the input from the movie input send a 
    // failedValidationResponse if any errors encountered during validation
    movieValidatorPtr := validator.New()
    
    data.ValidateMovie(movieValidatorPtr, &movie)
    if !movieValidatorPtr.Valid() {
        appPtr.failedValidationResponse(w, r, movieValidatorPtr.Errors)
        return
    }
    //Store the movie in our database
    err = appPtr.dbModel.MovieModel.InsertMovie(&movie)
    if err != nil {
        appPtr.serverErrorResponse(w, r, err)
        return
    }

    // When sending a HTTP response, we want to include a Location header to let the 
    // client know which URL they can find the newly-created resource at.
    headers := http.Header{}
    headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))
   
    //Return a response to the user that the movie was created successfully
    //the movie we are sending back will actually have been updated with the
    //fields that were erstwhile empty from the client, these fields have been
    //populated by our database and updated in the movie now being sent back
    err = appPtr.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
    if err != nil {
        appPtr.serverErrorResponse(w, r, err)
    }
}
/*********************************************************************************************************************/
//GET /v1/movies/:id
//To get info about a specific movie
func (appPtr *application) showMovieHandler (w http.ResponseWriter, r *http.Request) {
	//Get the value of the named parameter "id" from the request
    id, err := appPtr.readIDParam(r)
    if err != nil {
        // let the client know that we had a problem reading the id
        // provided in the req url, most likely, the provided id is invalid
        appPtr.badRequestResponse(w, r, fmt.Errorf("read id: %w", err))
        return
    }

    // Call the Get() method to fetch the data for a specific movie. We also need to 
    // use the errors.Is() function to check if it returns a data.ErrRecordNotFound
    // error, in which case we send a 404 Not Found response to the client
    // otherwise, we send a serverErrorResponse
    moviePtr, err := appPtr.dbModel.MovieModel.GetMovie(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            appPtr.notFoundHandler(w, r)
        default:
            appPtr.serverErrorResponse(w, r, err)
        }
        return
    }

    //wrap the movie data with the string "movie"
    wrappedMovieData := envelope{ "movie": *moviePtr }

    //marshal the movie data into json and send to the client
    err = appPtr.writeJSON(w, http.StatusOK, wrappedMovieData, nil) 

    //Respond with an error if we encountered an error marshalling the movie data into valid json
    if err != nil {
        //log error and send json-formatted error to client
        //log error if unable to format error to json and send empty response with
        //code 500 to client
        appPtr.serverErrorResponse(w, r, err)
    }
}
/*********************************************************************************************************************/
// PUT (UPDATE) /v1/movies/:id
func (appPtr *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
    //Create a new movie input struct
    var input data.MovieInput

    //Unmarshal the JSON from request body into the input struct
    //Send a bad request response if any error during unmarshaling
    err := appPtr.readJSON(w, r, &input)
    if err != nil {
        appPtr.badRequestResponse(w, r, err)
        return
    }

    //Get the value of the named parameter "id" from the request
    id, err := appPtr.readIDParam(r)
    if err != nil {
        // let the client know that we had a problem reading the id
        // provided in the req url, most likely, the provided id is invalid
        appPtr.badRequestResponse(w, r, fmt.Errorf("read id: %w", err))
        return
    }

    // Call the Get() method to fetch the data for a specific movie. We also need to 
    // use the errors.Is() function to check if it returns a data.ErrRecordNotFound
    // error, in which case we send a 404 Not Found response to the client
    // otherwise, we send a serverErrorResponse
    moviePtr, err := appPtr.dbModel.MovieModel.GetMovie(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            appPtr.notFoundHandler(w, r)
        default:
            appPtr.serverErrorResponse(w, r, err)
        }
        return
    }

    // Change the values of the movie we got back from the db to the new values
    // provided in the input from the request
    moviePtr.Title = input.Title
    moviePtr.Year = input.Year
    moviePtr.Runtime = input.Runtime
    moviePtr.Genres = input.Genres

    // Validate the input from the movie input send a 
    // failedValidationResponse if any errors encountered during validation
    movieValidatorPtr := validator.New()
    
    data.ValidateMovie(movieValidatorPtr, moviePtr)
    if !movieValidatorPtr.Valid() {
        appPtr.failedValidationResponse(w, r, movieValidatorPtr.Errors)
        return
    }

    //Store the new movie into the database
    //Send a not-found error if we cannot find for some strange reason the movie in the DB - 
    //Although this shouldn't happen, since the id we're using for the update was got from
    //the DB itself. We send a serverErrorResponse if we encountered any error updating the
    //resource successfully in the DB
    err = appPtr.dbModel.MovieModel.UpdateMovie(moviePtr)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            appPtr.notFoundHandler(w, r)
        default:
            appPtr.serverErrorResponse(w, r, err)
        }
        return
    }

    //RETURN THE UPDATED MOVIE
    //wrap the movie data with the string "movie"
    wrappedMovieData := envelope{ "movie": *moviePtr }

    //marshal the movie data into json and send to the client
    err = appPtr.writeJSON(w, http.StatusOK, wrappedMovieData, nil) 

    //Respond with an error if we encountered an error marshalling the movie data into valid json
    if err != nil {
        //log error and send json-formatted error to client
        //log error if unable to format error to json and send empty response with
        //code 500 to client
        appPtr.serverErrorResponse(w, r, err)
    }    
}
/*********************************************************************************************************************/
//DELETE /v1/movies/:id
//To delete a specific movie from the DB
func (appPtr application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
    //Get the value of the named parameter "id" from the request
    id, err := appPtr.readIDParam(r)
    if err != nil {
        // let the client know that we had a problem reading the id
        // provided in the req url, most likely, the provided id is invalid
        appPtr.badRequestResponse(w, r, fmt.Errorf("read id: %w", err))
        return
    }

    //Delete the movie from the DB
    moviePtr, err := appPtr.dbModel.MovieModel.Delete(id)

    if err != nil {
        switch {
            case errors.Is(err, data.ErrRecordNotFound):
                appPtr.notFoundHandler(w, r)
            default:
                appPtr.serverErrorResponse(w, r, err)
        }
        return
    }

    wrappedMovieData := envelope{"deleteOK": true, "movie": *moviePtr}
    err = appPtr.writeJSON(w, http.StatusOK, wrappedMovieData, nil)

    if err != nil {
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

3 - DEFINING METHODS ON APPPTR
Note that we can define methods on our appPtr in this file because the appPtr was declared in "package main", of which
ths file was also declared in the same package.
*/