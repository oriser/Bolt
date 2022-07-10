package db

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/oriser/bolt/testing/utils"
	userDomain "github.com/oriser/bolt/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userBuilder struct {
	user *userDomain.User
}

func (u *userBuilder) WithCustomID(id string) *userBuilder {
	u.user.ID = id
	return u
}
func (u *userBuilder) User() *userDomain.User {
	return u.user
}

func randomStringAlpha(n int) string {
	return utils.GenerateRandomString(append(utils.LowerLetters, utils.CapitalLetters...), n)
}

func randomStringNumeric(n int) string {
	return utils.GenerateRandomString(utils.NumberLetters, n)
}

func currentTimezone() string {
	currentTime := time.Now()
	zone, _ := currentTime.Zone()
	return zone
}

func getDummyUser() *userBuilder {
	return &userBuilder{&userDomain.User{
		FullName:           fmt.Sprintf("%s %s", randomStringAlpha(4), randomStringAlpha(3)),
		Email:              fmt.Sprintf("%s@%s.com", strings.ToLower(randomStringAlpha(4)), strings.ToLower(randomStringAlpha(4))),
		Phone:              fmt.Sprintf("%s-%s", randomStringNumeric(3), randomStringNumeric(7)),
		PaymentPreferences: nil,
		Timezone:           currentTimezone(),
		TransportID:        strings.ToUpper(randomStringAlpha(5)),
	}}
}

func testExpectedUsers(t *testing.T, expected []*userDomain.User, actual []*userDomain.User) {
	t.Helper()

	// For debugging to see the actual data
	derefActual := make([]userDomain.User, len(actual))
	for i, user := range actual {
		derefActual[i] = *user
	}

	require.Len(t, actual, len(expected))
	for _, expectedUser := range expected {
		found := false
		for _, actualUser := range actual {
			if assert.ObjectsAreEqual(expectedUser, actualUser) {
				found = true
				break
			}
		}
		require.True(t, found, "Expected to see user:\n%#v\nActual users:\n%#v", expectedUser, derefActual)
	}
}

func TestAddUser(t *testing.T) {
	t.Parallel()

	rand.Seed(time.Now().UnixNano())
	dbTest := NewDBTest(t)
	t.Cleanup(func() {
		dbTest.Cleanup(t)
	})

	tests := []struct {
		name           string
		users          []*userDomain.User
		listNamesCount int
	}{
		{
			name:           "Happy flow",
			users:          []*userDomain.User{getDummyUser().User()},
			listNamesCount: 1,
		},
		{
			name: "2 users filter just 1",
			users: []*userDomain.User{
				getDummyUser().User(),
				getDummyUser().User(),
			},
			listNamesCount: 1,
		},
		{
			name: "4 users filter 2",
			users: []*userDomain.User{
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
			},
			listNamesCount: 2,
		},
		{
			name: "6 users filter 6",
			users: []*userDomain.User{
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
				getDummyUser().User(),
			},
			listNamesCount: 6,
		},
		{
			name: "users with custom ID",
			users: []*userDomain.User{
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
				getDummyUser().WithCustomID(randomStringAlpha(5)).User(),
			},
			listNamesCount: 4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			//goland:noinspection ALL
			_, err := dbTest.db.db.Exec("DELETE FROM users")
			require.NoError(t, err)

			ctx := context.Background()
			// Adding users
			for _, user := range tc.users {
				id := user.ID
				err := dbTest.db.AddUser(ctx, user)
				require.NoError(t, err)

				assert.NotEmpty(t, user.ID)
				if id != "" {
					assert.Equal(t, id, user.ID)
				}
			}

			// Listing all users
			gotUsers, err := dbTest.db.ListUsers(ctx, userDomain.ListFilter{})
			require.NoError(t, err)

			assert.Len(t, gotUsers, len(tc.users))
			testExpectedUsers(t, tc.users, gotUsers)

			if tc.listNamesCount > 0 {
				// Listing specific users
				filterNames := make([]string, tc.listNamesCount)
				expectedUsers := make([]*userDomain.User, tc.listNamesCount)
				for i := 0; i < tc.listNamesCount; i++ {
					filterNames[i] = tc.users[i].FullName
					expectedUsers[i] = tc.users[i]
				}
				// Add dummy name that should not exists
				filterNames = append(filterNames, "some_dummy_name")

				gotUsers, err = dbTest.db.ListUsers(ctx, userDomain.ListFilter{Names: filterNames})
				require.NoError(t, err)

				assert.Len(t, gotUsers, tc.listNamesCount)
				// Checking that the users are equal
				testExpectedUsers(t, expectedUsers, gotUsers)
			}

			// Get specific user
			user, err := dbTest.db.GetUser(ctx, tc.users[0].ID)
			require.NoError(t, err)
			assert.Equal(t, tc.users[0], user)

			// Filter by TransportID
			users, err := dbTest.db.ListUsers(ctx, userDomain.ListFilter{TransportID: tc.users[0].TransportID})
			require.NoError(t, err)
			require.Len(t, users, 1)
			assert.Equal(t, tc.users[0], users[0])

			// Filter by TransportID and one name
			if len(tc.users) > 1 {
				transportID := tc.users[0].TransportID
				name := tc.users[1].FullName
				filter := userDomain.ListFilter{TransportID: transportID, Names: []string{name}}
				users, err = dbTest.db.ListUsers(ctx, filter)
				require.NoError(t, err)
				require.Len(t, users, 2)
				testExpectedUsers(t, []*userDomain.User{tc.users[0], tc.users[1]}, users)
			}
		})
	}
}
