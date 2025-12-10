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
func (appPtr *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
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
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
		Title:   input.Title,
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
func (appPtr *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
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
	wrappedMovieData := envelope{"movie": *moviePtr}

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
//PATCH (UPDATE) /v1/movies/:id
//To update a field in a specific movie
//Refer to notes(4) for more info on how null json values behave
func (appPtr *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	//Create a new movie input struct
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}

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
	// provided in the input from the request. Check individual fields if they
	// are nil (if the field is nil, then a value wasn't provided by the client
	// in the JSON they sent), if so, don't bother updating the value in the moviePtr
	// moviePtr.Title = input.Title
	// moviePtr.Year = input.Year
	// moviePtr.Runtime = input.Runtime
	// moviePtr.Genres = input.Genres

	if input.Title != nil {
		moviePtr.Title = *input.Title
	}
	if input.Year != nil {
		moviePtr.Year = *input.Year
	}
	if input.Runtime != nil {
		moviePtr.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		moviePtr.Genres = input.Genres
	}

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
		case errors.Is(err, data.ErrEditConflict):
			appPtr.editConflictResponse(w, r)
		default:
			appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//RETURN THE UPDATED MOVIE
	//wrap the movie data with the string "movie"
	wrappedMovieData := envelope{"movie": *moviePtr}

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
//To replace an entire movie with a given id in our database
func (appPtr *application) replaceMovieHandler(w http.ResponseWriter, r *http.Request) {
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
	// provided in the input from the request.
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
	wrappedMovieData := envelope{"movie": *moviePtr}

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

// GET /v1/movies
// To Get all the movies from the db: Also allows for filtering, sorting, and pagination
func (appPtr *application) showAllMoviesHandler(w http.ResponseWriter, r *http.Request) {
	//we'll define an input struct
	//to hold the expected values from the request query string.
	var input struct {
		Title   string
		Genres  []string
		Filters data.Filters
	}

	queryString := r.URL.Query()
	queryValidatorPtr := validator.New()

	// Use our helpers to extract the title and genres query string values, falling back
	// to defaults of an empty string and an empty slice respectively if they are not
	// provided by the client.
	input.Title = appPtr.readString(queryString, "title", "")
	input.Genres = appPtr.readCSV(queryString, "genres", []string{}, data.AllowedGenres, queryValidatorPtr)

	// Get the page and page_size query string values as integers. Notice that we set
	// the default page value to 1 and default page_size to 20, and that we pass the
	// validator instance as the final argument here.
	input.Filters.Page = appPtr.readInt(queryString, "page", 1, queryValidatorPtr)
	input.Filters.PageSize = appPtr.readInt(queryString, "page_size", 20, queryValidatorPtr)
	input.Filters.Sort = appPtr.readString(queryString, "sort", "id")

	//the values we allow to be provided as a value for the input.Filters.Sort field
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	data.ValidateFilters(queryValidatorPtr, input.Filters)

	// Check the Validator instance for any errors and use the failedValidationResponse()
	// helper to send the client a response if necessary.
	if !queryValidatorPtr.Valid() {
		appPtr.failedValidationResponse(w, r, queryValidatorPtr.Errors)
		return
	}

	// Call the GetAll() method to retrieve the movies,
	// At this point, we're 100% certain that whatever was
	// passed in the filter is valid, this is particularly important as in the GetAllMovies function
	// that is called, we don't do any safety checks on the filters - especially the sort field values.
	moviesPtrs, err := appPtr.dbModel.MovieModel.GetAllMovies(input.Title, input.Genres, input.Filters)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//dereference the individual moviePtrs in the moviesPtrs slice
	//add the actual movie to the moviesSlice. Note however that this
	//dereferencing is not necessary and is only here for clarity sake
	//Refer to Notes(5) for more on this
	moviesSlice := []data.Movie{}
	for _, moviePtr := range moviesPtrs {
		moviesSlice = append(moviesSlice, *moviePtr)
	}

	//total number of movies returned by the array including the
	//genre and title filters, not regarding the limit and offset
	//clauses.
	var totalRecords int
	if len(moviesSlice) == 0 {
		totalRecords = 0
	} else {
		totalRecords = moviesSlice[0].TotalMovies
	}

	moviesData := envelope{
		"metadata": data.CalculatePageMetadata(
			totalRecords,
			input.Filters.PageSize,
			input.Filters.Page,
		),
		"movies": moviesSlice,
	}

	err = appPtr.writeJSON(w, http.StatusOK, moviesData, nil)
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

4 - UPDATE MOVIE, SENDING NULL VALUES
When Go unmarshals our json body into the struct we provided, if we provide the value null for any field, it is ignored
it is as if we did not send any value for that field at all. So if we were to provide a value of null for a field
we wanted to update, the control-flow will look like:
- unmarshal json into input - nothing changes really, the null field is ignored
- change the movie from the db to fields in the input - again, nothing changes
- update the movie in the db - we update the movie in he db (increasing the version no.) when in reality, nothing changed

In an ideal world this type of request would return some kind of validation error. But — unless you write your own custom
JSON parser — there is no way to determine the difference between the client not supplying a key/value pair in the JSON,
or supplying it with the value null.

In most cases, it will probably suffice to explain this special-case behavior in client documentation for the endpoint
and say something like “JSON items with null values will be ignored and will remain unchanged”.

5 - POINTERS ARE ENCODED IN JSON AS THE VALUES POINTED TO
Pointers to a value are json encoded as the value that the pointer points to.
*/
