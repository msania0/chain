package appdb

import (
	"chain/database/pg"
	"chain/errors"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"

	"golang.org/x/net/context"
)

// Invitation represents an invitation to a core. It is intended to be
// used with API responses.
type Invitation struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Errors returned from functions in this file.
var (
	ErrInviteUserDoesNotExist = errors.New("invited user does not have an account")
)

const inviteIDBytes = 16

// CreateInvitation generates an invitation for an email address to join the
// core.
//
// The given email and role will be validated, and an error is returned if
// either is invalid.
func CreateInvitation(ctx context.Context, email, role string) (*Invitation, error) {
	email = strings.TrimSpace(email)
	role = strings.TrimSpace(role)

	if err := validateEmail(email); err != nil {
		return nil, err
	}

	if err := validateRole(role); err != nil {
		return nil, err
	}

	// Ensure that the email address is not already in use
	checkq := `
		SELECT 1 FROM users
		WHERE lower(email) = lower($1)
	`
	err := pg.QueryRow(ctx, checkq, email).Scan(new(int))
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "check user select query")
	}

	// Since IDs generated by next_chain_id() are relatively guessable, use an
	// ID that is not time-based.
	idRaw := make([]byte, inviteIDBytes)
	_, err = rand.Read(idRaw)
	if err != nil {
		return nil, errors.Wrap(err, "generate ID")
	}
	id := hex.EncodeToString(idRaw)

	insertq := `
		INSERT INTO invitations (id, email, role)
		VALUES ($1, $2, $3)
	`
	_, err = pg.Exec(ctx, insertq, id, email, role)
	if err != nil {
		return nil, errors.Wrap(err, "insert invitation query")
	}

	return &Invitation{
		ID:    id,
		Email: email,
		Role:  role,
	}, nil
}

// GetInvitation retrieves an Invitation from the database. If a
// matching invitation cannot be found, an error will be returned
// with pg.ErrUserInputNotFound as the root.
func GetInvitation(ctx context.Context, invID string) (*Invitation, error) {
	var (
		q = `
			SELECT email, role
			FROM invitations
			WHERE id = $1
		`
		email, role string
	)
	err := pg.QueryRow(ctx, q, invID).Scan(
		&email,
		&role,
	)
	if err == sql.ErrNoRows {
		return nil, errors.WithDetailf(pg.ErrUserInputNotFound, "invitation ID: %v", invID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "select query")
	}

	return &Invitation{
		ID:    invID,
		Email: email,
		Role:  role,
	}, nil
}

// CreateUserFromInvitation creates a new user account, using a user-supplied
// password and the email address and role contained in the invitation. If the
// user account is successfully created, the invitation will be deleted.
//
// The new user account will be validated with the same rules as CreateUser, and
// any validation failure will cause an error to be returned. If the invitation
// cannot be found, an error will be returned with pg.ErrUserInputNotFound as
// the root.
//
// The caller should ensure that the context contains a database transaction, or
// the function will panic.
func CreateUserFromInvitation(ctx context.Context, invID, password string) (*User, error) {
	_ = pg.FromContext(ctx).(pg.Tx) // panics if not in a db transaction

	inv, err := GetInvitation(ctx, invID)
	if err != nil {
		return nil, errors.Wrap(err, "get invitation")
	}

	user, err := CreateUser(ctx, inv.Email, password, inv.Role)
	if err != nil {
		return nil, errors.Wrap(err, "create user")
	}

	err = deleteInvitation(ctx, inv.ID)
	if err != nil {
		return nil, errors.Wrap(err, "delete invitation")
	}

	return user, nil
}

func deleteInvitation(ctx context.Context, invID string) error {
	q := `DELETE FROM invitations WHERE id = $1`
	_, err := pg.Exec(ctx, q, invID)
	if err != nil {
		return errors.Wrap(err, "delete query")
	}
	return nil
}
