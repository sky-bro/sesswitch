package session

import "testing"

func TestNonResumableRecordsOnlySurfaceReachableRequests(t *testing.T) {
	for _, kind := range []string{"needs_approval", "needs_input", "working", "turn_ended", "reviewed", "session_open", "idle", "closed", "saved", "error", "interrupted"} {
		for _, reachable := range []bool{false, true} {
			item := Session{Resume: "unavailable", State: State{Kind: kind}}
			if reachable {
				item.Location = &Location{Kind: "tmux"}
			}
			want := reachable && (kind == "needs_approval" || kind == "needs_input")
			if got := len(VisibleSessions([]Session{item})) == 1; got != want {
				t.Errorf("kind=%s reachable=%v visible=%v want=%v", kind, reachable, got, want)
			}
			item.Resume = ""
			if len(VisibleSessions([]Session{item})) != 1 {
				t.Fatalf("ordinary %s record hidden", kind)
			}
		}
	}
	marked := Session{Resume: "unavailable", State: State{Kind: "needs_input"}, Location: &Location{Kind: "tmux"}, Task: &Task{Kind: "done"}}
	if len(VisibleSessions([]Session{marked})) != 0 {
		t.Fatal("completed request remained visible")
	}
}
