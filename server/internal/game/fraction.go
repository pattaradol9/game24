package game

import "fmt"

// Fraction is an exact rational number used for all game arithmetic,
// so hands like 1,5,5,5 (5 * (5 - 1/5) = 24) are handled without float error.
type Fraction struct {
	Num int64
	Den int64 // always > 0
}

func F(n int64) Fraction { return Fraction{Num: n, Den: 1} }

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func (f Fraction) normalize() Fraction {
	if f.Den < 0 {
		f.Num, f.Den = -f.Num, -f.Den
	}
	g := gcd(f.Num, f.Den)
	if g > 1 {
		f.Num /= g
		f.Den /= g
	}
	return f
}

func (f Fraction) Add(o Fraction) Fraction {
	return Fraction{Num: f.Num*o.Den + o.Num*f.Den, Den: f.Den * o.Den}.normalize()
}

func (f Fraction) Sub(o Fraction) Fraction {
	return Fraction{Num: f.Num*o.Den - o.Num*f.Den, Den: f.Den * o.Den}.normalize()
}

func (f Fraction) Mul(o Fraction) Fraction {
	return Fraction{Num: f.Num * o.Num, Den: f.Den * o.Den}.normalize()
}

// Div returns an error on division by zero.
func (f Fraction) Div(o Fraction) (Fraction, error) {
	if o.Num == 0 {
		return Fraction{}, fmt.Errorf("division by zero")
	}
	return Fraction{Num: f.Num * o.Den, Den: f.Den * o.Num}.normalize(), nil
}

func (f Fraction) IsInt() bool           { return f.Den == 1 }
func (f Fraction) Is24() bool            { return f.Num == 24 && f.Den == 1 }
func (f Fraction) Equal(o Fraction) bool { return f.Num == o.Num && f.Den == o.Den }

// String renders "7" for integers and "1/5" for fractions.
func (f Fraction) String() string {
	if f.IsInt() {
		return fmt.Sprintf("%d", f.Num)
	}
	return fmt.Sprintf("%d/%d", f.Num, f.Den)
}
