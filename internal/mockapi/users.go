// Package mockapi package provides a mock API client for examples and demos.
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
var departments = []string{"HR", "IT", "Finance", "Marketing", "Sales", "Support", "Engineering", "Management"}
var users = make(map[int]User)

var mu sync.RWMutex

func init() {
	var adjs = []string{"Big", "Small", "Fast", "Slow", "Smart", "Happy", "Sad", "Funny", "Serious", "Angry"}
	var nouns = []string{"Dog", "Cat", "Bird", "Fish", "Mouse", "Elephant", "Lion", "Tiger", "Bear", "Wolf"}

	mu.Lock()
	defer mu.Unlock()

	// Generate users
	for i := 1; i <= 500; i++ {
		user := User{
			ID:         i,
			Name:       adjs[hash(i, "name1")%len(adjs)] + "_" + nouns[hash(i, "name2")%len(nouns)], // adj + noun
			Age:        hash(i, "age")%20 + 30,                                                      // 20-50
			Department: departments[hash(i, "dep")%len(departments)],                                // one of
			IsActive:   hash(i, "active")%100 < 60,                                                  // 60%
		}

		users[i] = user
	}
}

func GetDepartments() ([]string, error) {
	res := make([]string, len(departments))
	copy(res, departments)
	return res, nil
}

// GetUser returns a user by ID.
func GetUser(ctx context.Context, id int) (*User, error) {
	randomSleep(ctx, 1*time.Second)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mu.RLock()
	defer mu.RUnlock()

	user, ok := users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	return &user, nil
}

// GetUsers returns a list of users by IDs.
// If user is not found, nil is returned in the corresponding position.
func GetUsers(ctx context.Context, ids []int) ([]*User, error) {
	randomSleep(ctx, 2*time.Second)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mu.RLock()
	defer mu.RUnlock()

	res := make([]*User, 0, len(ids))
	for _, id := range ids {
		user, ok := users[id]
		if !ok {
			res = append(res, nil)
		} else {
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
	randomSleep(ctx, 3*time.Second)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

		res = append(res, &user)
	}

	return res, nil
}

// SaveUser saves a user.
func SaveUser(ctx context.Context, user *User) error {
	randomSleep(ctx, 3*time.Second)
	if err := ctx.Err(); err != nil {
		return err
	}

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

	users[user.ID] = *user
	return nil
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
