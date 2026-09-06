package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Ref points to an operand of a step: an original card or an earlier step result.
// The type is discriminated by which key is present in JSON: {"card":1} vs
// {"step":0} — a step index of 0 must not be mistaken for card 0.
type Ref struct {
	Card   int  `json:"card,omitempty"`
	Step   int  `json:"step,omitempty"`
	IsStep bool `json:"isStep,omitempty"`
}

// UnmarshalJSON discriminates card vs step refs by the presence of the key.
func (r *Ref) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["step"]; ok {
		f, err := toInt(v)
		if err != nil {
			return err
		}
		r.IsStep = true
		r.Step = f
		return nil
	}
	if v, ok := raw["card"]; ok {
		f, err := toInt(v)
		if err != nil {
			return err
		}
		r.IsStep = false
		r.Card = f
	}
	return nil
}

func toInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		return int(i), err
	default:
		return 0, fmt.Errorf("ref index must be a number")
	}
}

// Step is one merge of two values with an operator.
type Step struct {
	Left  Ref    `json:"left"`
	Right Ref    `json:"right"`
	Op    string `json:"op"` // "+", "-", "*", "/"
}

// Solution is one distinct way to reach 24.
type Solution struct {
	Expr        string `json:"expr"`
	IntegerOnly bool   `json:"integerOnly"`
	FirstStep   Step   `json:"firstStep"`
	// Trace is the full merge sequence; Verify accepts it as-is.
	Trace []Step `json:"-"`
}

type item struct {
	val      Fraction
	expr     string
	canon    string
	allInt   bool
	orig     []int // original card indexes composing this item, left to right
	size     int
	trace    []Step
	first    Step
	hasFirst bool
}

func (a item) merge(b item, op string) (item, bool) {
	var val Fraction
	switch op {
	case "+":
		val = a.val.Add(b.val)
	case "-":
		val = a.val.Sub(b.val)
	case "*":
		val = a.val.Mul(b.val)
	case "/":
		v, err := a.val.Div(b.val)
		if err != nil {
			return item{}, false
		}
		val = v
	default:
		return item{}, false
	}
	out := item{
		val:    val,
		expr:   "(" + a.expr + op + b.expr + ")",
		canon:  canonKey(op, a.canon, b.canon),
		allInt: a.allInt && b.allInt && val.IsInt(),
		orig:   append(append([]int{}, a.orig...), b.orig...),
		size:   a.size + b.size,
	}
	switch {
	case a.size == 1 && b.size == 1:
		out.first = Step{Left: Ref{Card: a.orig[0]}, Right: Ref{Card: b.orig[0]}, Op: op}
		out.hasFirst = true
	case a.hasFirst:
		out.first, out.hasFirst = a.first, true
	case b.hasFirst:
		out.first, out.hasFirst = b.first, true
	}
	// build the full evaluation trace: left subtrace, renumbered right
	// subtrace, then this merge
	offset := len(a.trace)
	out.trace = make([]Step, 0, offset+len(b.trace)+1)
	out.trace = append(out.trace, a.trace...)
	for _, st := range b.trace {
		st = remapSteps(st, offset)
		out.trace = append(out.trace, st)
	}
	var leftRef, rightRef Ref
	if a.size == 1 {
		leftRef = Ref{Card: a.orig[0]}
	} else {
		leftRef = Ref{Step: offset - 1, IsStep: true}
	}
	if b.size == 1 {
		rightRef = Ref{Card: b.orig[0]}
	} else {
		rightRef = Ref{Step: offset + len(b.trace) - 1, IsStep: true}
	}
	out.trace = append(out.trace, Step{Left: leftRef, Right: rightRef, Op: op})
	return out, true
}

func remapSteps(st Step, offset int) Step {
	if st.Left.IsStep {
		st.Left.Step += offset
	}
	if st.Right.IsStep {
		st.Right.Step += offset
	}
	return st
}

func canonKey(op, l, r string) string {
	if op == "+" || op == "*" {
		s := []string{l, r}
		sort.Strings(s)
		return "(" + s[0] + op + s[1] + ")"
	}
	return "(" + l + op + r + ")"
}

func merges(a, b item) []item {
	var out []item
	for _, op := range []string{"+", "-", "*", "/"} {
		if m, ok := a.merge(b, op); ok {
			out = append(out, m)
		}
	}
	// - and / are not commutative: enumerate the mirrored order too.
	for _, op := range []string{"-", "/"} {
		if m, ok := b.merge(a, op); ok {
			out = append(out, m)
		}
	}
	return out
}

// Solve returns every distinct solution for the four numbers, integer-only
// paths first. Duplicate derivations that differ only in commutative order
// are collapsed.
func Solve(numbers []int) []Solution {
	if len(numbers) != 4 {
		return nil
	}
	var items []item
	for i, n := range numbers {
		expr := fmt.Sprint(n)
		items = append(items, item{val: F(int64(n)), expr: expr, canon: expr, allInt: true, orig: []int{i}, size: 1})
	}
	var (
		out  []Solution
		seen = map[string]bool{}
	)
	var rec func(items []item)
	rec = func(items []item) {
		if len(items) == 1 {
			it := items[0]
			if it.val.Is24() && !seen[it.canon] {
				seen[it.canon] = true
				out = append(out, Solution{
					Expr:        strings.TrimSuffix(strings.TrimPrefix(it.expr, "("), ")"),
					IntegerOnly: it.allInt,
					FirstStep:   it.first,
					Trace:       it.trace,
				})
			}
			return
		}
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				a, b := items[i], items[j]
				rest := make([]item, 0, len(items)-2)
				for k := 0; k < len(items); k++ {
					if k != i && k != j {
						rest = append(rest, items[k])
					}
				}
				for _, m := range merges(a, b) {
					rec(append(append([]item{}, rest...), m))
				}
			}
		}
	}
	rec(items)
	sort.Slice(out, func(i, j int) bool {
		if out[i].IntegerOnly != out[j].IntegerOnly {
			return out[i].IntegerOnly
		}
		return len(out[i].Expr) < len(out[j].Expr)
	})
	return out
}

// HintStep is a suggested opening move.
type HintStep struct {
	LeftCard  int      `json:"leftCard"`
	RightCard int      `json:"rightCard"`
	Op        string   `json:"op"`
	Result    Fraction `json:"result"`
}

// Hint returns the first move of the most beginner-friendly solution.
func Hint(numbers []int) (HintStep, bool) {
	sols := Solve(numbers)
	for _, s := range sols {
		if s.IntegerOnly {
			return toHint(numbers, s.FirstStep)
		}
	}
	if len(sols) == 0 {
		return HintStep{}, false
	}
	return toHint(numbers, sols[0].FirstStep)
}

func toHint(numbers []int, st Step) (HintStep, bool) {
	a, b := F(int64(numbers[st.Left.Card])), F(int64(numbers[st.Right.Card]))
	var val Fraction
	switch st.Op {
	case "+":
		val = a.Add(b)
	case "-":
		val = a.Sub(b)
	case "*":
		val = a.Mul(b)
	case "/":
		v, err := a.Div(b)
		if err != nil {
			return HintStep{}, false
		}
		val = v
	}
	return HintStep{LeftCard: st.Left.Card, RightCard: st.Right.Card, Op: st.Op, Result: val}, true
}
