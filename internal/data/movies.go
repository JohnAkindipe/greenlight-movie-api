package data

import "time"

/*********************************************************************************************************************/
//MOVIE STRUCT
//This defines the data format for a movie in our API
/*
The - (hyphen) directive can be used when you never want a particular struct field to appear in the JSON output.
This is useful for fields that contain internal system information that isn’t relevant to your users, or sensitive
information that you don’t want to expose (like the hash of a password).

In contrast the omitempty directive hides a field in the JSON output if and only if the struct field value is empty,
where empty is defined as being:
- Equal to false, 0, or ""
- An empty array, slice or map
- A nil pointer or a nil interface value
*/
type Movie struct {
	ID        int64 		`json:"id"`
	CreatedAt time.Time		`json:"-"`
	Title string			`json:"title"`
	Year int32				`json:"year,omitempty"`
	Runtime Runtime 			`json:"runtime,omitempty"`//Movie runtime (in minutes) 
	Genres []string			`json:"genres,omitempty"`
	Version int32 			`json:"version"`//version number is initially 1 and will be incremented everytime
					//info about the movie is updated
}

