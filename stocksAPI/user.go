package stocksAPI

// general stuff:
import (
	//"math/big"
	"time"
)

// sqlite related imports:
import (
	"database/sql"
	//"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	UserID          UserID
	PrimaryEmail    string
	Name            string
	SecondaryEmails []string

	NotificationTimeout time.Duration
}

func (api *API) AddUser(user *User) (err error) {
	res, err := api.db.Exec(`insert into User (PrimaryEmail, Name, NotificationTimeout) values (?1,?2,?3)`, user.PrimaryEmail, user.Name, user.NotificationTimeout/time.Second)
	if err != nil {
		return err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	user.UserID = UserID(userID)

	if len(user.SecondaryEmails) > 0 {
		emails := make([][]interface{}, 0, len(user.SecondaryEmails))
		for _, e := range user.SecondaryEmails {
			emails = append(emails, []interface{}{e, user.UserID})
		}

		err := api.bulkInsert("UserEmail", []string{"Email", "UserID"}, emails)
		if err != nil {
			return err
		}
	}
	return
}

type dbUser struct {
	UserID              int64  `db:"UserID"`
	PrimaryEmail        string `db:"PrimaryEmail"`
	Name                string `db:"Name"`
	NotificationTimeout int    `db:"NotificationTimeout"`
}

func (api *API) projectUser(dbUser dbUser) (user *User, err error) {
	// get emails:
	emails := make([]struct {
		Email string `db:"Email"`
	}, 0, 2)
	err = api.db.Select(&emails, `select ue.Email from UserEmail as ue where ue.UserID = ?1`, dbUser.UserID)
	if err == sql.ErrNoRows {
		emails = make([]struct {
			Email string `db:"Email"`
		}, 0, 2)
	} else if err != nil {
		return
	}

	user = &User{
		UserID:              UserID(dbUser.UserID),
		PrimaryEmail:        dbUser.PrimaryEmail,
		Name:                dbUser.Name,
		NotificationTimeout: time.Duration(dbUser.NotificationTimeout) * time.Second,
	}

	user.SecondaryEmails = make([]string, 0, len(emails))
	for _, e := range emails {
		user.SecondaryEmails = append(user.SecondaryEmails, e.Email)
	}

	return
}

func (api *API) GetUser(userID UserID) (user *User, err error) {
	dbUser := dbUser{}

	// Get user by ID:
	err = api.db.Get(&dbUser, `select u.rowid as UserID, u.PrimaryEmail, u.Name, u.NotificationTimeout from User as u where u.rowid = ?1`, int64(userID))
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return
	}

	return api.projectUser(dbUser)
}

func (api *API) GetUserByEmail(email string) (user *User, err error) {
	dbUser := dbUser{}

	// Get user by primary or secondary email:
	err = api.db.Get(&dbUser, `select u.rowid as UserID, u.PrimaryEmail, u.Name, u.NotificationTimeout from User as u where u.PrimaryEmail = ?1
union all
select u.rowid as UserID, u.PrimaryEmail, u.Name, u.NotificationTimeout from User as u join UserEmail as ue on u.rowid = ue.UserID where ue.Email = ?1`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return
	}

	return api.projectUser(dbUser)
}
