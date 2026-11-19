package service

import (
	"encoding/json"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"
)

type UserService struct {
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

// checkPasswordStrength warns (without blocking) when a freshly set password
// is obviously weak.
func checkPasswordStrength(password string) string {
	if len(password) < 8 {
		return "password is shorter than 8 characters, consider using a stronger password"
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return "password lacks complexity (recommend using uppercase, lowercase, and digits)"
	}
	return ""
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return common.NewError("username can not be empty")
	} else if password == "" {
		return common.NewError("password can not be empty")
	}
	if warning := checkPasswordStrength(password); warning != "" {
		logger.Warning("Weak password for user '", username, "': ", warning)
	}
	hashedPass, err := util.HashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPass
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPass
	return db.Save(user).Error
}

func (s *UserService) Login(username string, password string, remoteIP string) (string, error) {
	if locked, remaining := LoginLockedOut(remoteIP); locked {
		logger.Warning("login refused, too many failures from ", remoteIP)
		return "", common.NewErrorf("too many failed attempts, try again in %d minute(s)",
			int(remaining.Minutes())+1)
	}

	user := s.CheckUser(username, password, remoteIP)
	if user == nil {
		if NoteLoginFailure(remoteIP) {
			logger.Warning("login locked out for ", remoteIP, " after ", maxLoginFailures, " failed attempts")
		}
		// The message stays the same whether the user exists or not.
		return "", common.NewError("wrong user or password! IP: ", remoteIP)
	}

	NoteLoginSuccess(remoteIP)
	return user.Username, nil
}

func (s *UserService) CheckUser(username string, password string, remoteIP string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if database.IsNotFound(err) {
		// Same work as a real check, so an unknown username cannot be told
		// apart from a known one by how long the answer takes.
		util.BurnPasswordCheck(password)
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err, " IP: ", remoteIP)
		return nil
	}

	if !util.CheckPassword(password, user.Password) {
		return nil
	}

	if !util.IsHashedPassword(user.Password) {
		if hashedPass, err := util.HashPassword(password); err == nil {
			if err := db.Model(model.User{}).Where("id = ?", user.Id).Update("password", hashedPass).Error; err != nil {
				logger.Warning("unable to upgrade stored password", err)
			} else {
				user.Password = hashedPass
			}
		}
	}

	lastLoginTxt := time.Now().Format("2006-01-02 15:04:05") + " " + remoteIP
	err = db.Model(model.User{}).
		Where("username = ?", username).
		Update("last_logins", &lastLoginTxt).Error
	if err != nil {
		logger.Warning("unable to log login data", err)
	}
	return user
}

func (s *UserService) GetUsers() (*[]model.User, error) {
	var users []model.User
	db := database.GetDB()
	err := db.Model(model.User{}).Select("id,username,last_logins").Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return &users, nil
}

// ChangePass rewrites the credentials of the logged-in user.
//
// It takes the username from the session rather than an id from the form. The
// id used to come straight from the request, so any authenticated user could
// rewrite the credentials of any other account by posting a different number,
// and with a single-admin panel that means taking the panel over.
func (s *UserService) ChangePass(loginUser string, oldPass string, newUser string, newPass string) error {
	if loginUser == "" {
		return common.NewError("not logged in")
	}
	if newUser == "" {
		return common.NewError("username can not be empty")
	}
	if newPass == "" {
		// Left unchecked, this stored a bcrypt hash of "" and the panel
		// authenticated anyone who submitted an empty password.
		return common.NewError("password can not be empty")
	}
	if warning := checkPasswordStrength(newPass); warning != "" {
		logger.Warning("Weak password for user '", newUser, "': ", warning)
	}

	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).Where("username = ?", loginUser).First(user).Error
	if err != nil {
		return err
	}
	if !util.CheckPassword(oldPass, user.Password) {
		return common.NewError("wrong password")
	}
	hashedPass, err := util.HashPassword(newPass)
	if err != nil {
		return err
	}
	user.Username = newUser
	user.Password = hashedPass
	return db.Save(user).Error
}

func (s *UserService) LoadTokens() ([]byte, error) {
	db := database.GetDB()
	var tokens []model.Tokens
	err := db.Model(model.Tokens{}).Preload("User").Where("expiry == 0 or expiry > ?", time.Now().Unix()).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, t := range tokens {
		result = append(result, map[string]interface{}{
			"token":    t.Token,
			"expiry":   t.Expiry,
			"username": t.User.Username,
		})
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return jsonResult, nil
}

func (s *UserService) GetUserTokens(username string) (*[]model.Tokens, error) {
	db := database.GetDB()
	var token []model.Tokens
	err := db.Model(model.Tokens{}).Select("id,desc,'****' as token,expiry,user_id").Where("user_id = (select id from users where username = ?)", username).Find(&token).Error
	if err != nil && !database.IsNotFound(err) {
		println(err.Error())
		return nil, err
	}
	return &token, nil
}

func (s *UserService) AddToken(username string, expiry int64, desc string) (string, error) {
	db := database.GetDB()
	var userId uint
	err := db.Model(model.User{}).Where("username = ?", username).Select("id").Scan(&userId).Error
	if err != nil {
		return "", err
	}
	if expiry > 0 {
		expiry = expiry*86400 + time.Now().Unix()
	}
	token := &model.Tokens{
		Token:  common.Random(32),
		Desc:   desc,
		Expiry: expiry,
		UserId: userId,
	}
	err = db.Create(token).Error
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

// DeleteToken removes one of the caller's own API tokens. The owner check is
// the point: the id came from the form with no constraint, so any logged-in
// user could revoke any other user's tokens by counting upwards.
func (s *UserService) DeleteToken(loginUser string, id string) error {
	if loginUser == "" {
		return common.NewError("not logged in")
	}
	db := database.GetDB()
	res := db.Where("id = ? AND user_id = (select id from users where username = ?)", id, loginUser).
		Delete(&model.Tokens{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("no such token")
	}
	return nil
}
