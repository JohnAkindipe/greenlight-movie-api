package main

import (
	"fmt"
	"net/http"
)

/*********************************************************************************************************************/
/*
The important thing to point out here is that healthcheckHandler is implemented as a method on our application struct.
This is an effective and idiomatic way to make dependencies available to our handlers without resorting to
global variables or closures — any dependency that the healthcheckHandler needs can simply be included as a field
in the application struct when we initialize it in main().

HANDLES GET /v1/healthcheck
*/
func (appPtr *application) healthcheckHandler (w http.ResponseWriter, r *http.Request) {
	/*
	//MY ATTEMPT to send JSON data
	data := map[string]string{
		"status": "available",
		"version": version,
		"environment": appPtr.config.env,
	}

	jsonData, err := json.Marshal(data)

	if err != nil {
		fmt.Printf("error marshaling data: %s/n", err)
	}

	w.Write(jsonData)
	*/
	// Set the "Content-Type: application/json" header on the response. If you forget to
    // this, Go will default to sending a "Content-Type: text/plain; charset=utf-8"
    // header instead.
	w.Header().Set("Content-Type", "application/json")

	// Create a fixed-format JSON response from a string. Notice how we're using a raw
    // string literal (enclosed with backticks) so that we can include double-quote 
    // characters in the JSON without needing to escape them? We also use the %q verb to 
    // wrap the interpolated values in double-quotes.
	js := `{"status": "available", "environment": %q, "version": %q}`
    js = fmt.Sprintf(js, appPtr.config.env, version)
	w.Write([]byte(js))
}