package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

/*********************************************************************************************************************/
// CUSTOM TYPE TO ENVELOPE RESPONSES
type envelope map[string]any

/*********************************************************************************************************************/
//HELPER TO EXTRACT NAMED PARAMETERS FROM A REQUEST

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

	for key, value := range headers {
        w.Header()[key] = value
    }

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