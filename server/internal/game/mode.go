package game

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// Mode is a difficulty tier named after face cards.
type Mode string

const (
	Jack  Mode = "jack"
	Queen Mode = "queen"
	King  Mode = "king"
	Ace   Mode = "ace"
)

// ModeConfig holds the per-mode rules.
type ModeConfig struct {
	Min, Max     int // number range dealt
	TimeLimitSec int // countdown per hand
	Multiplier   int // EXP multiplier
}

func Config(m Mode) (ModeConfig, error) {
	switch m {
	case Jack:
		return ModeConfig{Min: 1, Max: 10, TimeLimitSec: 120, Multiplier: 1}, nil
	case Queen:
		return ModeConfig{Min: 1, Max: 13, TimeLimitSec: 90, Multiplier: 2}, nil
	case King:
		return ModeConfig{Min: 1, Max: 13, TimeLimitSec: 75, Multiplier: 3}, nil
	case Ace:
		return ModeConfig{Min: 1, Max: 19, TimeLimitSec: 60, Multiplier: 5}, nil
	default:
		return ModeConfig{}, fmt.Errorf("unknown mode %q", m)
	}
}

func AllModes() []Mode { return []Mode{Jack, Queen, King, Ace} }

// MustConfig panics on unknown mode; use only for validated modes.
func MustConfig(m Mode) ModeConfig {
	c, err := Config(m)
	if err != nil {
		panic(err)
	}
	return c
}

// ValidateMode reports whether the dealt hand satisfies the mode's promise:
//   - Jack: hands are solvable with integer-only intermediate steps
//   - Queen: hands are solvable
//   - King: hands are solvable but require fractional intermediate steps
//   - Ace: hands have exactly one solution and require fractions
func ValidateMode(m Mode, numbers []int) error {
	sols := Solve(numbers)
	integerPath := false
	for _, s := range sols {
		if s.IntegerOnly {
			integerPath = true
			break
		}
	}
	switch m {
	case Jack:
		if !integerPath {
			return errors.New("hand has no integer-only solution")
		}
	case Queen:
		if len(sols) == 0 {
			return errors.New("hand has no solution")
		}
	case King:
		if len(sols) == 0 {
			return errors.New("hand has no solution")
		}
		if integerPath {
			return errors.New("hand has an integer-only solution")
		}
	case Ace:
		if len(sols) != 1 {
			return fmt.Errorf("hand has %d solutions, want exactly 1", len(sols))
		}
		if sols[0].IntegerOnly {
			return errors.New("hand has an integer-only solution")
		}
	default:
		return fmt.Errorf("unknown mode %q", m)
	}
	return nil
}

const maxDealAttempts = 200000

// Deal draws four numbers (with replacement) that satisfy the mode.
func Deal(m Mode, rnd *rand.Rand) ([]int, error) {
	cfg, err := Config(m)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < maxDealAttempts; attempt++ {
		numbers := []int{
			cfg.Min + rnd.IntN(cfg.Max-cfg.Min+1),
			cfg.Min + rnd.IntN(cfg.Max-cfg.Min+1),
			cfg.Min + rnd.IntN(cfg.Max-cfg.Min+1),
			cfg.Min + rnd.IntN(cfg.Max-cfg.Min+1),
		}
		if ValidateMode(m, numbers) == nil {
			return numbers, nil
		}
	}
	return nil, fmt.Errorf("could not deal a valid %s hand after %d attempts", m, maxDealAttempts)
}
