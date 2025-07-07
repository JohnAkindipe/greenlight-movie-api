package data

import (
	"database/sql"
	"errors"
)

// Define a custom ErrRecordNotFound error. We'll return this from our Get() method when
// looking up a movie that doesn't exist in our database.
var (
    ErrRecordNotFound = errors.New("record not found")
    ErrEditConflict = errors.New("edit conflict")
)
/*********************************************************************************************************************/
/*
MODELS
We’re going to wrap our MovieModel in a parent Models struct.  it has the benefit of giving you a convenient single
‘container’ which can hold and represent all your database models as your application grows.
*/
// type Models struct {
// 	movieModel *MovieModel
// }

// Create a Models struct which wraps the MovieModel. We'll add other models to this,
// like a UserModel and PermissionModel, as our build progresses.
type Models struct {
    MovieModel MovieModel
    UserModel UserModel
}

/*
CREATE NEW MODEL
For ease of use, we also add a New() method which returns a Models struct containing
the initialized MovieModel. Notice that the function is called NewModel but returns
a "Models" object
*/
func NewModel(dbPtr *sql.DB) Models {
    return Models{
        MovieModel: MovieModel{DBPtr: dbPtr},
        UserModel: UserModel{DBPtr: dbPtr},
    }
}