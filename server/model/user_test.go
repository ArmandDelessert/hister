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

func TestUserRevisionsTrackAdministrativeChanges(t *testing.T) {
	testutil.InitModel(t)
	admin, err := model.CreateUser("admin", "password123", true)
	if err != nil {
		t.Fatal(err)
	}
	user := testutil.CreateUser(t, "alice")
	changes := []func() error{
		func() error { return model.UpdateUsername("alice", "renamed") },
		func() error { return model.UpdatePassword("renamed", "replacement123") },
		func() error { _, err := model.RegenerateToken(user.ID); return err },
		func() error { _, err := model.ToggleAdmin("renamed"); return err },
	}
	for _, change := range changes {
		if err := change(); err != nil {
			t.Fatal(err)
		}
		if _, err := model.UpdateUser(admin.ID, user.ID, user.Revision, model.UserChanges{RegenerateToken: true}); !errors.Is(err, model.ErrStaleUserRevision) {
			t.Fatalf("outdated update was not rejected: %v", err)
		}
		current, err := model.GetUserByID(user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Revision != user.Revision+1 {
			t.Fatalf("revision = %d, want %d", current.Revision, user.Revision+1)
		}
		user = current
	}
}

func TestConcurrentUserUpdates(t *testing.T) {
	// A real database and multiple connections exercise database locking, rather
	// than hiding a race behind an in-memory database with one connection.
	testutil.InitModelWithConfig(t, testutil.Config(t))
	admin, err := model.CreateUser("admin", "password123", true)
	if err != nil {
		t.Fatal(err)
	}
	user := testutil.CreateUser(t, "alice")
	start, results := make(chan struct{}), make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := model.UpdateUser(admin.ID, user.ID, user.Revision, model.UserChanges{RegenerateToken: true})
			results <- err
		}()
	}
	close(start)
	var success, stale int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			success++
		case errors.Is(err, model.ErrStaleUserRevision):
			stale++
		default:
			t.Errorf("concurrent update failed unexpectedly: %v", err)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("success = %d, stale = %d", success, stale)
	}
}

func TestConcurrentLastAdminRemoval(t *testing.T) {
	for _, otherOperation := range []string{"patch", "toggle", "delete", "delete by ID"} {
		t.Run(otherOperation, func(t *testing.T) {
			testutil.InitModelWithConfig(t, testutil.Config(t))
			first, err := model.CreateUser("first", "password123", true)
			if err != nil {
				t.Fatal(err)
			}
			second, err := model.CreateUser("second", "password123", true)
			if err != nil {
				t.Fatal(err)
			}
			start, results := make(chan struct{}), make(chan error, 2)
			go func() {
				<-start
				no := false
				_, err := model.UpdateUser(first.ID, first.ID, first.Revision, model.UserChanges{IsAdmin: &no})
				results <- err
			}()
			go func() {
				<-start
				var err error
				switch otherOperation {
				case "patch":
					no := false
					_, err = model.UpdateUser(second.ID, second.ID, second.Revision, model.UserChanges{IsAdmin: &no})
				case "toggle":
					_, err = model.ToggleAdmin(second.Username)
				case "delete":
					err = model.DeleteUser(second.Username)
				case "delete by ID":
					err = model.DeleteUserByID(second.ID)
				}
				results <- err
			}()
			close(start)
			var success, blocked int
			for range 2 {
				err := <-results
				switch {
				case err == nil:
					success++
				case errors.Is(err, model.ErrLastAdmin):
					blocked++
				default:
					t.Errorf("unexpected removal error: %v", err)
				}
			}
			var admins int64
			if err := model.DB.Model(&model.User{}).Where("is_admin = ?", true).Count(&admins).Error; err != nil {
				t.Fatal(err)
			}
			if success != 1 || blocked != 1 || admins != 1 {
				t.Fatalf("success = %d, blocked = %d, administrators = %d", success, blocked, admins)
			}
		})
	}
}

func TestUpdateUserRechecksAdministrator(t *testing.T) {
	testutil.InitModel(t)
	admin, err := model.CreateUser("admin", "password123", true)
	if err != nil {
		t.Fatal(err)
	}
	user := testutil.CreateUser(t, "alice")
	for _, actorID := range []uint{0, user.ID, user.ID + 1} {
		if _, err := model.UpdateUser(actorID, admin.ID, admin.Revision, model.UserChanges{RegenerateToken: true}); !errors.Is(err, model.ErrAdminRequired) {
			t.Fatalf("unauthorized actor %d update: %v", actorID, err)
		}
	}
}

func TestTokenRotationOwnership(t *testing.T) {
	testutil.InitModel(t)
	admin, err := model.CreateUser("admin", "password123", true)
	if err != nil {
		t.Fatal(err)
	}
	alice := testutil.CreateUser(t, "alice")
	bob := testutil.CreateUser(t, "bob")
	for _, actor := range []uint{0, bob.ID, bob.ID + 1} {
		if _, err := model.RotateUserToken(actor, alice.ID, 1); !errors.Is(err, model.ErrAdminRequired) {
			t.Fatalf("unauthorized rotation: %v", err)
		}
	}
	if _, err := model.UpdateUser(alice.ID, alice.ID, 1, model.UserChanges{RegenerateToken: true}); !errors.Is(err, model.ErrAdminRequired) {
		t.Fatalf("self service weakened administrative updates: %v", err)
	}
	rotated, err := model.RotateUserToken(alice.ID, alice.ID, 1)
	if err != nil || rotated.Revision != 2 || rotated.Token == alice.Token {
		t.Fatalf("self rotation: %v", err)
	}
	if _, err := model.RotateUserToken(alice.ID, alice.ID, 1); !errors.Is(err, model.ErrStaleUserRevision) {
		t.Fatalf("stale self rotation: %v", err)
	}
	rotated, err = model.RotateUserToken(admin.ID, alice.ID, 2)
	if err != nil || rotated.Revision != 3 {
		t.Fatalf("admin rotation: %v", err)
	}
	if _, err := model.ToggleAdmin("bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := model.ToggleAdmin("admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := model.RotateUserToken(admin.ID, alice.ID, 3); !errors.Is(err, model.ErrAdminRequired) {
		t.Fatalf("demoted administrator still rotated someone else's token: %v", err)
	}
}
