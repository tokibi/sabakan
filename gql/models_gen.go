// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package gql

import (
	"github.com/cybozu-go/sabakan"
)

// Label represents an arbitrary key-value pairs.
type Label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// LabelInput represents a label to search machines.
type LabelInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MachineParams is a set of input parameters to search machines.
type MachineParams struct {
	Labels              []LabelInput           `json:"labels"`
	Racks               []int                  `json:"racks"`
	Roles               []string               `json:"roles"`
	States              []sabakan.MachineState `json:"states"`
	MinDaysBeforeRetire *int                   `json:"minDaysBeforeRetire"`
}