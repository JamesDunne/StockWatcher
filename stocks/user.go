package stocks

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
	UserID UserID
	Name   string
	Emails []UserEmail

	NotificationTimeout time.Duration
}

type UserEmail struct {
	Email     string
	UserID    UserID
	IsPrimary bool
}

func (u *User) PrimaryEmail() string {
	for _, e := range u.Emails {
		if e.IsPrimary {
			return e.Email
		}
	}
	return ""
}

func (api *API) AddUser(user *User) (err error) {
	res, err := api.db.Exec(`insert into User (Name, NotificationTimeout) values (?1,?2)`, user.Name, user.NotificationTimeout/time.Second)
	if err != nil {
		return err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	user.UserID = UserID(userID)

	if len(user.Emails) > 0 {
		emails := make([][]interface{}, 0, len(user.Emails))
		for _, e := range user.Emails {
			emails = append(emails, []interface{}{e.Email, user.UserID, e.IsPrimary})
		}

		err := api.bulkInsert("UserEmail", []string{"Email", "UserID", "IsPrimary"}, emails)
		if err != nil {
			return err
		}
	}
	return
}

type dbUser struct {
	UserID              int64  `db:"UserID"`
	Name                string `db:"Name"`
	NotificationTimeout int    `db:"NotificationTimeout"`
}

type dbUserEmail struct {
	Email     string `db:"Email"`
	IsPrimary int64  `db:"IsPrimary"`
}

func (api *API) projectUser(dbUser dbUser) (user *User, err error) {
	// get emails:
	emails := make([]dbUserEmail, 0, 2)
	err = api.db.Select(&emails, `select Email, IsPrimary from UserEmail where UserID = ?1`, dbUser.UserID)
	if err == sql.ErrNoRows {
		emails = make([]dbUserEmail, 0, 2)
	} else if err != nil {
		return
	}

	user = &User{
		UserID:              UserID(dbUser.UserID),
		Name:                dbUser.Name,
		NotificationTimeout: time.Duration(dbUser.NotificationTimeout) * time.Second,
		Emails:              make([]UserEmail, 0, len(emails)),
	}

	for _, e := range emails {
		user.Emails = append(user.Emails, UserEmail{
			Email:     e.Email,
			IsPrimary: fromDbBool(e.IsPrimary),
		})
	}

	return
}

func (api *API) GetUser(userID UserID) (user *User, err error) {
	dbUser := dbUser{}

	// Get user by ID:
	err = api.db.Get(&dbUser, `select UserID, Name, NotificationTimeout from User where UserID = ?1`, int64(userID))
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return
	}

	return api.projectUser(dbUser)
}

func (api *API) GetUserByEmail(email string) (user *User, err error) {
	dbUser := dbUser{}

	// Get user by email:
	err = api.db.Get(&dbUser, `
select u.UserID, u.Name, u.NotificationTimeout
from User as u
join UserEmail as ue on u.UserID = ue.UserID
where ue.Email = ?1`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return
	}

	return api.projectUser(dbUser)
}
