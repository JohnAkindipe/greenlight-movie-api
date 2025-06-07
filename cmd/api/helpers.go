package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
*/