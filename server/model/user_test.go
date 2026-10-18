// SPDX-License-Identifier: AGPL-3.0-or-later

package model_test

import (
	"errors"
	"testing"

	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestUpdatePassword(t *testing.T) {
	testutil.InitModel(t)
	testutil.CreateUser(t, "alice")

	if err := model.UpdatePassword("alice", "newpassword123"); err != nil {
		t.Fatal(err)
	}
	if _, err := model.AuthenticateUser("alice", "password123"); !errors.Is(err, model.ErrInvalidPassword) {
		t.Fatalf("old password authentication error = %v, want %v", err, model.ErrInvalidPassword)
	}
	if _, err := model.AuthenticateUser("alice", "newpassword123"); err != nil {
		t.Fatalf("new password authentication failed: %v", err)
	}

	user, err := model.GetUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Password == "newpassword123" {
		t.Fatal("updated password was stored as plain text")
	}
}

func TestUpdatePasswordUserNotFound(t *testing.T) {
	testutil.InitModel(t)

	err := model.UpdatePassword("missing", "newpassword123")
	if !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("UpdatePassword error = %v, want %v", err, model.ErrUserNotFound)
	}
}

func TestDeleteUserByIDAfterRename(t *testing.T) {
	testutil.InitModel(t)
	original := testutil.CreateUser(t, "alice")
	if err := model.UpdateUsername("alice", "renamed"); err != nil {
		t.Fatal(err)
	}
	replacement := testutil.CreateUser(t, "alice")
	if err := model.DeleteUserByID(original.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := model.GetUserByID(original.ID); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("original user lookup = %v, want user not found", err)
	}
	if _, err := model.GetUserByID(replacement.ID); err != nil {
		t.Fatalf("deletion affected the replacement account: %v", err)
	}
	for _, id := range []uint{0, original.ID, replacement.ID + 1} {
		if err := model.DeleteUserByID(id); !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("DeleteUserByID(%d) = %v, want user not found", id, err)
		}
	}
}

func TestGetUserByIDDatabaseError(t *testing.T) {
	testutil.InitModel(t)
	user := testutil.CreateUser(t, "alice")
	db, err := model.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := model.GetUserByID(user.ID); err == nil || errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("database failure reported as missing user: %v", err)
	}
}

func TestCreateUserConcurrentDuplicate(t *testing.T) {
	testutil.InitModel(t)
	db, err := model.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Keep the in-memory SQLite database on a single connection.
	db.SetMaxOpenConns(1)
	const attempts = 4
	start := make(chan struct{})
	results := make(chan error, attempts)
	for range attempts {
		go func() {
			<-start
			_, err := model.CreateUser("alice", "password123", false)
			results <- err
		}()
	}
	close(start)
	created := 0
	for range attempts {
		err := <-results
		if err == nil {
			created++
		} else if !errors.Is(err, model.ErrUserAlreadyExists) {
			t.Errorf("duplicate creation error = %v, want %v", err, model.ErrUserAlreadyExists)
		}
	}
	if created != 1 {
		t.Fatalf("created %d accounts, want 1", created)
	}
	if _, err := model.AuthenticateUser("alice", "password123"); err != nil {
		t.Fatalf("created account cannot authenticate: %v", err)
	}
}
