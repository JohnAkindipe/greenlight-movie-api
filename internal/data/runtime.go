package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

/*********************************************************************************************************************/
/*
CUSTOM RUNTIME TYPE
Define a custom type for the Runtime, so that we can define a custom MarshalJSON method for it which will be called
when our program tries to marshal any data represented as the runtime type into JSON.
*/
type Runtime int32
/*********************************************************************************************************************/
// Define an error that our UnmarshalJSON() method can return if we're unable to parse
// or convert the JSON string successfully.
var ErrInvalidRuntimeFormat = errors.New(`runtime should be in the format: "<runtime> mins", where runtime is a valid int`)
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

/*********************************************************************************************************************/
/*
CUSTOM UNMARSHALJSON FUNC
Refer to notes for info on challenges i faced debugging this issue
*/
func (rPtr *Runtime) UnmarshalJSON(jsonForm []byte) error {
	var stringForm string

	//unmarshal the json value into a string
	if err := json.Unmarshal(jsonForm, &stringForm); err != nil {
		return ErrInvalidRuntimeFormat
	}
	
	//check if stringform has suffix " mins", return an error if it doesn't
	if !strings.HasSuffix(stringForm, " mins") {
		return ErrInvalidRuntimeFormat
	}

	//trim the " mins" suffix from stringform, it should now have simply a number
	//in string form i.e from "56 mins" to "56"
	stringForm = strings.TrimSuffix(stringForm, " mins")
	
	//Convert string to valid int e.g. "56" to 56, return an error if we can't convert
	//the string representation to a valid int. It means the client did not send a valid 
	//integer for the runtime value
	intForm, err := strconv.ParseInt(stringForm, 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	//set the value that the ptr points to as the runtime
	*rPtr = Runtime(intForm)
	return nil
}

/*********************************************************************************************************************/
/*
NOTES:
1. UNMARSHAL JSON FUNC
- I ran into some issues implementing this function:
i) I was initially turning the json slice of bytes to string using the string() method, although there were no compile-time
warnings, turns out, converting json bytes to string this way, makes whatever string we get highly incompatible with the
way we would expect strings to work, ergo, when i was asking if the string has a suffix of "90 mins", I kept getting false
despite glaring evidence that it did indeed have the suffix " mins", the issue is converting json bytes to string using
the strings method, actually converts them to ""90 mins"", hence when i was printing the string to terminal during debugging
i got "90 mins" (printing strings to terminal will automatically remove the double quotes around them). The fact in the
preceding bracket was not apparent at the time, and it had me fooled, i thought the string was "90 mins", when it was
actually ""90 mins"". the solution is to use json.Unmarshal to unmarshal the json bytes into a string, this way the string
is "90 mins" and will behave as we expect (will be printed as 90 mins on terminal -- double quotes removed).
ii) The other issue was that after doing all the conversions, i was setting the runtime to the new conversion, but it
didn't seem to work, i was confused and thought the UnmarshalJSON should allow me return a value that will be set for the
field whose custom UnmarshalJSON it called, but this was not the case. I was actually right in setting the RUNTIME value
to the value I had extracted from the json ("90 mins" -> 90), the problem was it was not reflecting because I implemented the
UnmarshalJSON method using a value receiver, therefore the method was operating on a copy of the value, thus any change I made
to this value, was not evident outside the method, simple yet tricky. The solution was to declare it as a pointer receiver,
that way, when the method is called, an address to the runtime value is passed, thus any change i make in the UnmarshalJSON
method, actually changes the value outside the method. Phew, I learnt a lot.
*/