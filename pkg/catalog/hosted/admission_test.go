package hosted

import (
	"strconv"
	"sync"
	"testing"
)

func TestSixCallsAndOnePerUser(t *testing.T) {
	rt := &Runtime{active: map[string]int{}}
	var wg sync.WaitGroup
	admitted := make(chan string, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := strconv.Itoa(i)
			if rt.admit(user, 1, 6) {
				admitted <- user
			}
		}()
	}
	wg.Wait()
	close(admitted)
	users := []string{}
	for user := range admitted {
		users = append(users, user)
	}
	if len(users) != 6 {
		t.Fatalf("admitted %d, want 6", len(users))
	}
	if rt.admit(users[0], 1, 6) {
		t.Fatal("same user admitted twice")
	}
	rt.release(users[0])
	if !rt.admit("next", 1, 6) {
		t.Fatal("finished slot not reusable")
	}
	// Also prove the per-user cap independently of global saturation.
	rt = &Runtime{active: map[string]int{}}
	if !rt.admit("one", 1, 6) || rt.admit("one", 1, 6) {
		t.Fatal("one user exceeded one active call")
	}
}
