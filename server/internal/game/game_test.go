package game

import (
	"math/rand/v2"
	"testing"
)

func TestFractionArithmetic(t *testing.T) {
	a, b := F(1), F(5)
	if q, err := a.Div(b); err != nil || q.String() != "1/5" {
		t.Fatalf("1/5 = %v (%v)", q, err)
	}
	oneFifth, _ := a.Div(b)
	if got := F(1).Sub(oneFifth); got.String() != "4/5" {
		t.Fatalf("1-1/5 = %s", got)
	}
	if got := b.Mul(b.Sub(oneFifth)); !got.Is24() {
		t.Fatalf("5*(5-1/5) = %s, want 24", got)
	}
	if _, err := F(1).Div(F(0)); err == nil {
		t.Fatal("expected division by zero error")
	}
}

func TestSolveKnownHands(t *testing.T) {
	tests := []struct {
		name        string
		numbers     []int
		wantSome    bool
		wantIntPath bool
		minCount    int
	}{
		{"classic 4,7,8,8", []int{4, 7, 8, 8}, true, true, 1},
		{"impossible 1,1,1,1", []int{1, 1, 1, 1}, false, false, 0},
		{"fractional 1,5,5,5", []int{1, 5, 5, 5}, true, false, 1},
		{"fractional 3,3,8,8", []int{3, 3, 8, 8}, true, false, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sols := Solve(tc.numbers)
			if tc.wantSome && len(sols) < tc.minCount {
				t.Fatalf("Solve(%v) = %d solutions, want >= %d", tc.numbers, len(sols), tc.minCount)
			}
			if !tc.wantSome && len(sols) != 0 {
				t.Fatalf("Solve(%v) = %d solutions, want 0", tc.numbers, len(sols))
			}
			intPath := false
			for _, s := range sols {
				if s.IntegerOnly {
					intPath = true
				}
				// every solution's own trace must verify
				if err := Verify(tc.numbers, s.Trace); err != nil {
					t.Fatalf("solution trace %q failed verify: %v", s.Expr, err)
				}
			}
			if intPath != tc.wantIntPath {
				t.Fatalf("integer path = %v, want %v", intPath, tc.wantIntPath)
			}
		})
	}
}

func TestVerifyTrace(t *testing.T) {
	numbers := []int{4, 7, 8, 8}
	// (7 - 8/8) * 4 = 24: cards are [4,7,8,8]
	trace := []Step{
		{Left: Ref{Card: 2}, Right: Ref{Card: 3}, Op: "/"},
		{Left: Ref{Card: 1}, Right: Ref{Step: 0, IsStep: true}, Op: "-"},
		{Left: Ref{Step: 1, IsStep: true}, Right: Ref{Card: 0}, Op: "*"},
	}
	if err := Verify(numbers, trace); err != nil {
		t.Fatalf("valid trace rejected: %v", err)
	}
	if got := ExprFromSteps(numbers, trace); got != "((7-(8/8))*4)" {
		t.Fatalf("expr = %s", got)
	}

	bad := []struct {
		name  string
		trace []Step
	}{
		{"reuse card", []Step{
			{Left: Ref{Card: 1}, Right: Ref{Card: 1}, Op: "+"},
			{Left: Ref{Card: 0}, Right: Ref{Card: 2}, Op: "+"},
			{Left: Ref{Step: 1, IsStep: true}, Right: Ref{Card: 3}, Op: "+"},
		}},
		{"wrong result", []Step{
			{Left: Ref{Card: 0}, Right: Ref{Card: 1}, Op: "+"},
			{Left: Ref{Step: 0, IsStep: true}, Right: Ref{Card: 2}, Op: "+"},
			{Left: Ref{Step: 1, IsStep: true}, Right: Ref{Card: 3}, Op: "+"},
		}},
		{"too few steps", []Step{
			{Left: Ref{Card: 0}, Right: Ref{Card: 1}, Op: "+"},
		}},
		{"reuse step", []Step{
			{Left: Ref{Card: 0}, Right: Ref{Card: 1}, Op: "+"},
			{Left: Ref{Step: 0, IsStep: true}, Right: Ref{Card: 2}, Op: "+"},
			{Left: Ref{Step: 0, IsStep: true}, Right: Ref{Card: 3}, Op: "+"},
		}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if err := Verify(numbers, tc.trace); err == nil {
				t.Fatal("invalid trace accepted")
			}
		})
	}
}

func TestDealSatisfiesModes(t *testing.T) {
	for _, m := range AllModes() {
		t.Run(string(m), func(t *testing.T) {
			rnd := rand.New(rand.NewPCG(42, 24))
			cfg, err := Config(m)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 5; i++ {
				hand, err := Deal(m, rnd)
				if err != nil {
					t.Fatalf("deal: %v", err)
				}
				for _, n := range hand {
					if n < cfg.Min || n > cfg.Max {
						t.Fatalf("hand %v out of range [%d,%d]", hand, cfg.Min, cfg.Max)
					}
				}
				if err := ValidateMode(m, hand); err != nil {
					t.Fatalf("dealt hand violates mode: %v (%v)", err, hand)
				}
			}
		})
	}
}

func TestHint(t *testing.T) {
	h, ok := Hint([]int{4, 7, 8, 8})
	if !ok {
		t.Fatal("expected hint")
	}
	nums := []int{4, 7, 8, 8}
	if h.Step.LeftCard == h.Step.RightCard {
		t.Fatalf("hint uses same card twice: %v", h)
	}
	a, b := F(int64(nums[h.Step.LeftCard])), F(int64(nums[h.Step.RightCard]))
	var want Fraction
	switch h.Step.Op {
	case "+":
		want = a.Add(b)
	case "-":
		want = a.Sub(b)
	case "*":
		want = a.Mul(b)
	case "/":
		q, err := a.Div(b)
		if err != nil {
			t.Fatalf("hint divides by zero: %v", h)
		}
		want = q
	default:
		t.Fatalf("unexpected op %q", h.Step.Op)
	}
	if !want.Equal(h.Step.Result) {
		t.Fatalf("hint result %s, want %s", h.Step.Result, want)
	}
	sols := Solve(nums)
	if h.Expr == "" {
		t.Fatal("hint carries no equation")
	}
	if h.Count != len(sols) || len(h.Alternatives) != len(sols)-1 {
		t.Fatalf("solution count = %d with %d alternatives, want %d and %d",
			h.Count, len(h.Alternatives), len(sols), len(sols)-1)
	}
	if _, ok := Hint([]int{1, 1, 1, 1}); ok {
		t.Fatal("unsolvable hand should have no hint")
	}
}
