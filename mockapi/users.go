// Package mockapi provides a very basic mock API for examples and demos.
// It's intentionally kept public to enable running and experimenting with examples in the Go Playground.
// The implementation is naive and uses full scan for all operations.
package mockapi

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"
)

type User struct {
	ID         int
	Name       string
	Age        int
	Department string
	IsActive   bool
}

// don't use pointers here, to make sure that raw data is not accessible from outside
var departments []string
var users []User

var mu sync.RWMutex

func init() {
	const usersCount = 100

	var adjs = []string{"Big", "Small", "Fast", "Slow", "Smart", "Happy", "Sad", "Funny", "Serious", "Angry"}
	var nouns = []string{"Dog", "Cat", "Bird", "Fish", "Mouse", "Elephant", "Lion", "Tiger", "Bear", "Wolf"}

	mu.Lock()
	defer mu.Unlock()

	departments = []string{"HR", "IT", "Finance", "Marketing", "Sales", "Support", "Engineering", "Management"}

	// Generate users
	// Use deterministic values for all fields to make examples reproducible
	users = make([]User, 0, usersCount)

	for i := 1; i <= usersCount; i++ {
		user := User{
			ID:         i,
			Name:       adjs[hash(i, "name1")%len(adjs)] + " " + nouns[hash(i, "name2")%len(nouns)], // adj + noun
			Age:        hash(i, "age")%20 + 30,                                                      // 20-50
			Department: departments[hash(i, "dep")%len(departments)],                                // one of
			IsActive:   hash(i, "active")%100 < 60,                                                  // 60%
		}

		users = append(users, user)
	}
}

func GetDepartments() ([]string, error) {
	res := make([]string, len(departments))
	copy(res, departments)
	return res, nil
}

// GetUser returns a user by ID.
func GetUser(ctx context.Context, id int) (*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	randomSleep(ctx, 500*time.Millisecond)

	mu.RLock()
	defer mu.RUnlock()

	idx, err := getUserIndex(id)
	if err != nil {
		return nil, err
	}

	user := users[idx]
	return &user, nil
}

// GetUsers returns a list of users by IDs.
// If a user is not found, nil is returned in the corresponding position.
func GetUsers(ctx context.Context, ids []int) ([]*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	randomSleep(ctx, 1000*time.Millisecond)

	mu.RLock()
	defer mu.RUnlock()

	res := make([]*User, 0, len(ids))
	for _, id := range ids {
		idx, err := getUserIndex(id)
		if err != nil {
			res = append(res, nil)
		} else {
			user := users[idx]
			res = append(res, &user)
		}
	}

	return res, nil
}

type UserQuery struct {
	Department string
	Page       int
}

// ListUsers returns a paginated list of users optionally filtered by department.
func ListUsers(ctx context.Context, query *UserQuery) ([]*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	randomSleep(ctx, 1000*time.Millisecond)

	const pageSize = 10
	if query == nil {
		query = &UserQuery{}
	}

	offset := query.Page * pageSize

	mu.RLock()
	defer mu.RUnlock()

	res := make([]*User, 0, 10)
	for _, user := range users {
		if query.Department != "" && user.Department != query.Department {
			continue
		}

		if offset > 0 {
			offset--
			continue
		}

		if len(res) >= pageSize {
			break
		}

		userCopy := user
		res = append(res, &userCopy)
	}

	return res, nil
}

// SaveUser saves a user.
func SaveUser(ctx context.Context, user *User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	randomSleep(ctx, 1000*time.Millisecond)

	if user == nil {
		return fmt.Errorf("user is nil")
	}

	if user.Name == "" {
		return fmt.Errorf("username is empty")
	}
	if user.Age <= 0 {
		return fmt.Errorf("age is invalid")
	}

	mu.Lock()
	defer mu.Unlock()

	idx, err := getUserIndex(user.ID)
	if err != nil {
		users = append(users, *user)
	} else {
		users[idx] = *user
	}

	return nil
}

func getUserIndex(id int) (int, error) {
	for i, u := range users {
		if u.ID == id {
			return i, nil
		}
	}

	return -1, fmt.Errorf("user not found")
}

func hash(input ...any) int {
	hasher := fnv.New32()
	fmt.Fprintln(hasher, input...)
	return int(hasher.Sum32())
}

func randomSleep(ctx context.Context, max time.Duration) {
	dur := time.Duration(rand.Intn(int(max)))
	t := time.NewTimer(dur)
	defer t.Stop()

	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
