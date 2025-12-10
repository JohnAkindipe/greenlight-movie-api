package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Define a Permissions slice, which we will use to hold the permission codes (like
// "movies:read" and "movies:write") for a single user.
type Permissions []string

type PermissionModel struct {
	DBPtr *sql.DB
}

// Add a helper method to check whether the Permissions slice contains a specific
// permission code.
func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}
	return false
}

// The GetAllForUser() method returns all permission codes for a specific user in a
// Permissions slice. The code in this method should feel very familiar --- it uses the
// standard pattern that we've already seen before for retrieving multiple data rows in
// an SQL query.

func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
	query := `
        SELECT permissions.code
        FROM permissions
        INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
        INNER JOIN users ON users_permissions.user_id = users.id
        WHERE users.id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DBPtr.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := Permissions{}
	for rows.Next() {
		var permission string
		err = rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (m PermissionModel) AddForUser(userID int64, permissions ...string) error {
	if len(permissions) < 1 {
		return errors.New("must supply at least one permission")
	}

	query := `
		INSERT INTO users_permissions(user_id, permission_id)
		VALUES ($1, (SELECT id FROM permissions WHERE code = $2))
		`

	if len(permissions) > 1 {
		stringBuilder := strings.Builder{}
		for i := range permissions {
			if i == 0 {
				stringBuilder.WriteString(query)
				continue
			}
			stringBuilder.WriteString(
				fmt.Sprintf(",\n($1, (SELECT id FROM permissions WHERE code = $%d))", i+2),
			)
		}

		query = strings.TrimSpace(stringBuilder.String())
	}

	query += ";"
	args := make([]any, len(permissions)+1)
	args[0] = userID
	for i, v := range permissions {
		args[i+1] = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DBPtr.ExecContext(ctx, query, args...)
	return err
}
