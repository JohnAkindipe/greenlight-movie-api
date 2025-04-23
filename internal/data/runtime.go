package data

import (
	"encoding/json"
	"fmt"
	"strconv"
)

/*********************************************************************************************************************/
/*
CUSTOM RUNTIME TYPE
Define a custom type for the Runtime, so that we can define a custom MarshalJSON method for it which will be called
when our program tries to marshal any data represented as the runtime type into JSON.
*/
type Runtime int32

/*********************************************************************************************************************/
/*CUSTOM MARSHALJSON FUNC*/
func (r Runtime) MarshalJSON() ([]byte, error) {

	//Convert r into a string
	stringForm := strconv.FormatInt(int64(r), 10)
	// strconv.Itoa()
	//Format the returned string into a custom string e.g. "64 mins"
	finalRep := fmt.Sprintf("%s mins", stringForm)
	fmt.Println(finalRep)
	return json.Marshal(finalRep)
}