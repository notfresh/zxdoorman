package doorman

import (
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	store := NewLeaseStore("test")
	store.Assign("c1", 3*time.Second, time.Second, 10, 12)
	store.Assign("c2", 3*time.Second, time.Second, 10, 12)
	store.Assign("c3", 5*time.Second, time.Second, 15, 20)

	if want, got := 35, store.SumHas(); want != got {
		t.Errorf("store SumHas() %v want %v", got, want)
	}

	if want, got := 44, store.SumWant(); want != got {
		t.Errorf("store SumWant() %v want %v", got, want)
	}

	if want, got := 10, store.Get("c1").Has; want != got {
		t.Errorf("store SumHas() %v want %v", got, want)
	}

	time.Sleep(3 * time.Second)
	store.Clean()
	if want, got := 15, store.SumHas(); want != got {
		t.Errorf("store SumHas() %v want %v", got, want)
	}

	if want, got := 20, store.SumWant(); want != got {
		t.Errorf("store SumWant() %v want %v", got, want)
	}

	if got := store.Get("c1"); !got.IsZero() {
		t.Errorf("lease for client c1 is %v", got)
	}

}
