package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

/*********************************************************************************************************************/
// CUSTOM TYPE TO ENVELOPE RESPONSES
type envelope map[string]any

/*********************************************************************************************************************/
//HELPER TO EXTRACT NAMED PARAMETERS FROM A REQUEST
/*
func getNamedParam(name string, r *http.Request) string {

	// When httprouter is parsing a request, any interpolated URL parameters will be
	// stored in the request context. We can use the ParamsFromContext() function to
	// retrieve a slice containing these parameter names and values
	params := httprouter.ParamsFromContext(r.Context())

	// We can then use the ByName() method to get the value of the "name" parameter from
	// the slice.
	param := params.ByName(name)

	return param
}
*/

/*********************************************************************************************************************/
// RETRIEVE THE ID URL PARAMETER FROM THE CURRENT REQUEST CONTEXT
// Retrieve the "id" URL parameter from the current request context, then convert it to
// an integer and return it. If the operation isn't successful, return 0 and an error.
func (appPtr *application) readIDParam(r *http.Request) (int64, error) {
	// When httprouter is parsing a request, any interpolated URL parameters will be
	// stored in the request context. We can use the ParamsFromContext() function to
	// retrieve a slice containing these parameter names and values
	params := httprouter.ParamsFromContext(r.Context())

	// We can then use the ByName() method to get the value of the "id" parameter from
	// the slice.
	// In our project all movies will have a unique positive integer ID, but
	// the value returned by params.ByName is always a string. So we try to convert it to a
	// base 10 integer (with a bit size of 64). If the parameter couldn't be converted,
	// or is less than 1, we know the ID is invalid, return 0 and an error.
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

/*********************************************************************************************************************/
//WRITE JSON HELPER
func (appPtr *application) writeJSON(w http.ResponseWriter, status int, wrappedData envelope, headers http.Header) error {
	wrappedJSONData, err := json.MarshalIndent(wrappedData, "", "\t")
	if err != nil {
		return err
	}
	// Append a newline to the JSON. This is just a small nicety to make it easier to
	// view in terminal applications.
	wrappedJSONData = append(wrappedJSONData, '\n')

	// range over the headers parameter and set the response headers as specified
	// in the header parameter
	for key, value := range headers {
		w.Header()[key] = value
	}

	/*
	   It is important that we have not written headers when there is a possibility of errors happening.
	   We have designed the code to only write to the response stream, when we can guarantee that no
	   errors can occur from our operation on the data we want to send.
	*/
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// w.Write(jsonData)
	w.Write(wrappedJSONData)

	return nil
	/*********************************************************************************************************************/
	/*
			USING FIXED-FORMAT JSON
			// Create a fixed-format JSON response from a string. Notice how we're using a raw
		    // string literal (enclosed with backticks) so that we can include double-quote
		    // characters in the JSON without needing to escape them? We also use the %q verb to
		    // wrap the interpolated values in double-quotes.
			js := `{"status": "available", "environment": %q, "version": %q}`
		    js = fmt.Sprintf(js, appPtr.config.env, version)
			w.Write([]byte(js))
	*/
}

/*********************************************************************************************************************/
/*
READ JSON
We're going to use this functino to read json from requests and send respnoses as appropriate
Although the dest in readJson has been marked as having an any type, it actually should be a
pointer to any type i.e *any
*/
func (appPtr *application) readJSON(w http.ResponseWriter, r *http.Request, dest any) error {

	//Prevent request body being > 1MB i.e (1,048,576 bytes)
	r.Body = http.MaxBytesReader(w, r.Body, int64(1_048_576))

	//Read the request body and Decode the request body into movieInput struct
	//Send an error response if errors decoding
	bodyDecoder := json.NewDecoder(r.Body)

	//prevent random unallowed fields from being silently ignored, return an error instead
	bodyDecoder.DisallowUnknownFields()

	err := bodyDecoder.Decode(dest)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError //Refer to questions(2)
		var maxBytesError *http.MaxBytesError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unallowed fields: %s", fieldName)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("request body has exceeded limit: %d bytes", maxBytesError.Limit)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	//Prevent request body from having more than json content per request
	//barring any other thing but the one JSON body we expect
	err = bodyDecoder.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("expect request to contain only one JSON body")
	}
	return nil
}

// The readString() helper returns a string value from the query string, or the provided
// default value if no matching key could be found.
// The readString() helper returns a string value from the query string, or the provided
// default value if no matching key could be found.
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	// Extract the value for a given key from the query string. If no key exists this
	// will return the empty string "".
	s := qs.Get(key)

	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}

	// Otherwise return the string.
	return s
}

func (appPtr *application) readInt(qs url.Values, key string, defaultValue int, queryValidatorPtr *validator.Validator) int {
	value := qs.Get(key)

	if value == "" {
		return defaultValue
	}

	//intVal is not a valid number and can't be converted to an int
	intVal, err := strconv.Atoi(value)
	queryValidatorPtr.Check(
		err == nil,
		key,
		"must be a number",
	)
	if err != nil {
		return defaultValue
	}

	// //intVal is a valid number but is negative
	// queryValidatorPtr.Check(
	//     intVal >= 0,
	//     key,
	//     fmt.Sprintf("%s cannot be negative", intVal),
	// )
	return intVal
}

// The readCSV() helper reads a string value from the query string and then splits it
// into a slice on the comma character. If no matching key could be found, it returns
// the provided default value.
// This function allows some callers to provide a value for validatorPtr and allowedValues
// or not (passing nil instead for both params), however, if a value is passed for validatorPtr
// a value must be passed for allowedValues, otherwise the function may behave unexpectedly
func (app *application) readCSV(qs url.Values, key string, defaultValue []string, allowedValues []string, validatorPtr *validator.Validator) []string {
	// Extract the value from the query string.
	csv := qs.Get(key)

	// If no key exists (or the value is empty) then return the default value.
	if csv == "" {
		return defaultValue
	}

	// Otherwise parse the value into a []string slice
	sliceVal := strings.Split(csv, ",")

	//this will allow some callers of this function to pass nil as a value of validatorPtr
	//meaning they don't care about validation.
	if validatorPtr != nil {
		for _, sliceElem := range sliceVal {
			validatorPtr.Check(
				slices.Contains(allowedValues, sliceElem),
				key,
				fmt.Sprintf("must contain elements included in the array: %+v", allowedValues),
			)
			if _, exists := validatorPtr.Errors[key]; exists {
				return sliceVal //TODO: would it make sense to return sliceVal or defaultValue
			}
		}
	}

	return sliceVal
}

func (appPtr *application) PseudoreadCSV(key string, queryValidatorPtr *validator.Validator, r *http.Request) []string {
	queryParams := r.URL.Query()
	value := queryParams.Get(key) //value is probably somn like "crime,action"

	csvSlice := strings.Split(value, ",") //now split into "[crime, action]"

	//have to check if all the elements in the array is a valid genre
	for _, csvElem := range csvSlice {
		queryValidatorPtr.Check(
			validator.PermittedValue(csvElem, data.AllowedGenres...),
			key,
			fmt.Sprintf("csvElem is not a valid %s value", key),
		)
	}

	return csvSlice
}

// We would ideally call this function using the "go" keyword
// so that it runs in a separate goroutine, someFunc will
// run and any panics will be handled by the defer statement.
// Refer notes(2) on benefits of putting waitgroup in "background"
func (appPtr *application) background(fn func()) {
	appPtr.wg.Add(1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				appPtr.logger.Error(fmt.Sprintf("%v", err))
			}
			//the author calls done in a separate call to defer
			appPtr.wg.Done()
		}()
		//we cannot run this func in its own goroutine
		//when a goroutine panics, only deferred statements
		//inside the goroutine itself can recover panics
		fn()
	}()
}

/*********************************************************************************************************************/
/*
QUESTION:
1. What is the difference between errors.Is and errors.As
2. Why are we taking a pointer to a pointer in arguments to errors.As.
   var ae *argError
    if errors.As(err, &ae)

Consider the above golang code, ignore the fact that the snippet is incomplete.
I have noticed that in go code, when comparing errors using errors.As, a nil pointer
is usually initialized which is a pointer to a type which implements the error interface, then we pass the address
of this pointer as a second argument to errors.As, in essence, why must this be the case, in fact, it is considered
an error to merely pass in the pointer value (i.e. pass in ae as the second parameter) as the second parameter to
errors.As, it is usually passed as address of pointer value. In essence we are passing the address of an address.
 Why is this the case?

 NOTES
 1. CALLING WG.ADD IN BACKGRUND
 It makes logical sense to call wg.Add() in background instead of from the calling function (as I did before). By
 encapsulatiing all the logic of calling goroutines in our entire application in one place, we reduce the need for
 repetition, as well as eleminate the possibility of forgetting to call the wg.Add(1) from the calling function.
 All the logic for launching background goroutines in our app is in "background", including adding to the waitgroup,
 calling the goroutine and so on. A simple call to "background" thus handles all the logistics.
*/
