package covenantsigner

import (
	"context"
	"errors"
)

var errJobNotFound = errors.New("covenant signer job not found")

type Transition struct {
	State          JobState
	Detail         string
	Reason         FailureReason
	PSBTHash       string
	TransactionHex string
	Handoff        map[string]any
}

type Engine interface {
	OnSubmit(ctx context.Context, job *Job) (*Transition, error)
	OnPoll(ctx context.Context, job *Job) (*Transition, error)
}

type passiveEngine struct{}

func NewPassiveEngine() Engine {
	return &passiveEngine{}
}

func (pe *passiveEngine) OnSubmit(context.Context, *Job) (*Transition, error) {
	return &Transition{
		State:  JobStatePending,
		Detail: "accepted for covenant signing",
	}, nil
}

func (pe *passiveEngine) OnPoll(context.Context, *Job) (*Transition, error) {
	return nil, nil
}
