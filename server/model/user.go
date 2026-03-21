// SPDX-FileContributor: Adam Tauber <asciimoo@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
)

type User struct {
	CommonFields
	Username string `gorm:"uniqueIndex" json:"username"`
	Password string `json:"-"`
}

func CreateUser(username, password string) error {
	var existing User
	if err := DB.Where("username = ?", username).First(&existing).Error; err == nil {
		return ErrUserAlreadyExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return DB.Create(&User{Username: username, Password: string(hash)}).Error
}

func DeleteUser(username string) error {
	result := DB.Where("username = ?", username).Delete(&User{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func AuthenticateUser(username, password string) (*User, error) {
	var u User
	if err := DB.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}
	return &u, nil
}
