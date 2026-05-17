package workflow

import "testing"

func TestNormalizeTaskState(t *testing.T) {
	cases := map[string]string{
		"doing":    StateInProgress,
		"qa":       StateReview,
		"backlog":  StateTodo,
		"assigned": StateAssigned,
		"":         StateTodo,
	}
	for in, want := range cases {
		if got := NormalizeTaskState(in); got != want {
			t.Fatalf("NormalizeTaskState(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateTransition(t *testing.T) {
	valid := []TransitionRequest{
		{FromState: StateTodo, ToState: StateAssigned},
		{FromState: StateAssigned, ToState: StateInProgress},
		{FromState: StateInProgress, ToState: StateReview},
		{FromState: StateReview, ToState: StateDone, ReviewAction: ReviewApprove},
		{FromState: StateReview, ToState: StateInProgress, ReviewAction: ReviewRequestChange},
		{FromState: StateDone, ToState: StateInProgress, ReviewAction: ReviewReopen},
		{FromState: StateInProgress, ToState: StateBlocked, Reason: "waiting on dependency"},
	}
	for _, req := range valid {
		if _, err := ValidateTransition(req); err != nil {
			t.Fatalf("expected valid transition %+v, got err %v", req, err)
		}
	}

	invalid := []TransitionRequest{
		{FromState: StateTodo, ToState: StateReview},
		{FromState: StateAssigned, ToState: StateDone},
		{FromState: StateBlocked, ToState: StateDone},
		{FromState: StateReview, ToState: StateDone},
		{FromState: StateInProgress, ToState: StateBlocked},
	}
	for _, req := range invalid {
		if _, err := ValidateTransition(req); err == nil {
			t.Fatalf("expected invalid transition %+v", req)
		}
	}
}
