package validator

import (
	"cmp"
	"regexp"
	"slices"
)

/*
EMAIL REGEX
Declare a regular expression for sanity checking the format of email addresses (we'll
use this later in the book). This regular expression pattern is taken from
https://html.spec.whatwg.org/#valid-e-mail-address.
*/
var (
    EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

/*********************************************************************************************************************/
//VALIDATOR STRUCT
type Validator struct {
	Errors map[string]string
}

/*
CREATE NEW VALIDATOR
*/
func New() *Validator {
	return &Validator{ Errors: make(map[string]string)}
}

/*
IS VALIDATOR VALID?
*/
func (vPtr *Validator) Valid() bool {
	return len(vPtr.Errors) == 0
}

/*
ADD ERROR
AddError adds an error message to the VALIDATOR.ERRORS map (so long as no entry already exists for
the given key).
*/
func (vPtr *Validator) AddError(key, errorMsg string) {
	if _, exists := vPtr.Errors[key]; !exists{
		vPtr.Errors[key] = errorMsg
	}
}

/*
CHECK
Check adds an error message to the map only if a validation check is not 'ok'.
*/
func (vPtr *Validator) Check(ok bool, key, message string) {
	if !ok {
		vPtr.AddError(key, message)
	}
}
/*********************************************************************************************************************/
/* 
PERMITTED VALUES
Generic function which returns true if a specific value is in a list of permitted values.
*/
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	return slices.Contains(permittedValues, value)
}

/* 
MATCHES
Matches returns true if a string value matches a specific regexp pattern.
*/
func Matches(value string, regexPattern *regexp.Regexp) bool {
	return regexPattern.MatchString(value)
}

/* 
UNIQUE
Generic function which returns true if all values in a slice are unique.
*/
func Unique[T cmp.Ordered](sliceOfValues []T) bool {
	slices.Sort(sliceOfValues)
	/*
	The slice is sorted, compare every element in the array with the next element
	if they are the same, they are indeed duplicates. sorting slice is very important.
	*/
	for i, value := range sliceOfValues {
		if i == len(sliceOfValues) - 1 { continue }
		if value == sliceOfValues[i + 1] {
			return false
		}
	}
	return true
}
/*********************************************************************************************************************/
/*
NOTES
1 - UNIQUE
Below is the function for unique used by the author in let's go further
func Unique[T comparable](values []T) bool {
    uniqueValues := make(map[T]bool)

    for _, value := range values {
        uniqueValues[value] = true
    }

    return len(values) == len(uniqueValues)
}
*/