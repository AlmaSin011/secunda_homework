package entity

import (
	"errors"
	"fmt"
)

type TaskStatus string

const (
	TaskTodo       TaskStatus = "todo"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
)

func (s TaskStatus) String() string { return string(s) }

var ValidStatuses = []TaskStatus{TaskTodo, TaskInProgress, TaskDone}

var ErrInvalidTaskStatus = errors.New("invalid task status")

func ParseTaskStatus(s string) (TaskStatus, error) {
	switch TaskStatus(s).toLower() {
	case TaskTodo:
		return TaskTodo, nil
	case TaskInProgress:
		return TaskInProgress, nil
	case TaskDone:
		return TaskDone, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidTaskStatus, s)
	}
}

func (s TaskStatus) IsValid() bool {
	for _, v := range ValidStatuses {
		if s == v {
			return true
		}
	}
	return false
}

func (s TaskStatus) toLower() TaskStatus {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return TaskStatus(b)
}
