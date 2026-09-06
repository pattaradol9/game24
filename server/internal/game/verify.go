package game

import "fmt"

// Verify replays the player's merge trace against the dealt hand.
// Every original card must be used exactly once, every intermediate result
// at most once, and the final value must be 24.
func Verify(numbers []int, steps []Step) error {
	if len(numbers) != 4 {
		return fmt.Errorf("need exactly 4 numbers, got %d", len(numbers))
	}
	if len(steps) != len(numbers)-1 {
		return fmt.Errorf("need %d steps, got %d", len(numbers)-1, len(steps))
	}
	usedCard := make([]bool, len(numbers))
	usedStep := make([]bool, len(steps))
	vals := make([]Fraction, len(steps))
	active := len(numbers)

	for i, st := range steps {
		var left, right Fraction
		var err error
		if left, err = resolve(st.Left, usedCard, usedStep, numbers, vals); err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
		if right, err = resolve(st.Right, usedCard, usedStep, numbers, vals); err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
		switch st.Op {
		case "+":
			vals[i] = left.Add(right)
		case "-":
			vals[i] = left.Sub(right)
		case "*":
			vals[i] = left.Mul(right)
		case "/":
			v, err := left.Div(right)
			if err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}
			vals[i] = v
		default:
			return fmt.Errorf("step %d: unknown operator %q", i, st.Op)
		}
		active = active - 2 + 1
	}
	if active != 1 {
		return fmt.Errorf("trace leaves %d unused values", active-1)
	}
	if !vals[len(vals)-1].Is24() {
		return fmt.Errorf("final value is %s, not 24", vals[len(vals)-1])
	}
	return nil
}

func resolve(r Ref, usedCard []bool, usedStep []bool, numbers []int, vals []Fraction) (Fraction, error) {
	if r.IsStep {
		if r.Step < 0 || r.Step >= len(vals) {
			return Fraction{}, fmt.Errorf("unknown step %d", r.Step)
		}
		if usedStep[r.Step] {
			return Fraction{}, fmt.Errorf("step %d already used", r.Step)
		}
		usedStep[r.Step] = true
		return vals[r.Step], nil
	}
	if r.Card < 0 || r.Card >= len(numbers) {
		return Fraction{}, fmt.Errorf("unknown card %d", r.Card)
	}
	if usedCard[r.Card] {
		return Fraction{}, fmt.Errorf("card %d already used", r.Card)
	}
	usedCard[r.Card] = true
	return F(int64(numbers[r.Card])), nil
}

// ExprFromSteps renders a readable expression from a verified trace.
func ExprFromSteps(numbers []int, steps []Step) string {
	cardStrs := make([]string, len(numbers))
	for i, n := range numbers {
		cardStrs[i] = fmt.Sprint(n)
	}
	stepStrs := make([]string, len(steps))
	for i, st := range steps {
		stepStrs[i] = "(" + refStr(st.Left, cardStrs, stepStrs) + st.Op + refStr(st.Right, cardStrs, stepStrs) + ")"
	}
	return stepStrs[len(steps)-1]
}

func refStr(r Ref, cardStrs, stepStrs []string) string {
	if r.IsStep {
		return stepStrs[r.Step]
	}
	return cardStrs[r.Card]
}
