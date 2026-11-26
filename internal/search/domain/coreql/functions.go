package coreql

import (
	"sort"
	"strings"
)

// ArgKind is the kind of argument a function accepts (uniform for all its arguments).
type ArgKind int

const (
	ArgText    ArgKind = iota + 1 // text such as a team name
	ArgRelDate                    // relative date offset such as -1d
)

// FuncSpec is the signature of a CoreQL function.
type FuncSpec struct {
	Name    string
	MinArgs int
	MaxArgs int
	Arg     ArgKind   // kind of every argument; zero when MaxArgs is 0
	Returns FieldType // type of the value the function produces
}

// FunctionRegistry resolves function names. Lookup is case-insensitive.
type FunctionRegistry interface {
	Lookup(name string) (FuncSpec, bool)
	Names() []string
}

type funcRegistry struct {
	specs map[string]FuncSpec
	names []string
}

// DefaultFunctions returns the built-in CoreQL functions.
func DefaultFunctions() FunctionRegistry { return defaultRegistry }

var defaultRegistry = newFuncRegistry([]FuncSpec{
	{Name: "currentUser", Returns: TypeUser},
	{Name: "membersOf", MinArgs: 1, MaxArgs: 1, Arg: ArgText, Returns: TypeUser},
	{Name: "now", Returns: TypeDateTime},
	{Name: "startOfDay", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "endOfDay", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "startOfWeek", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "endOfWeek", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "startOfMonth", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "endOfMonth", MaxArgs: 1, Arg: ArgRelDate, Returns: TypeDateTime},
	{Name: "openSprints", Returns: TypeSprint},
	{Name: "closedSprints", Returns: TypeSprint},
	{Name: "futureSprints", Returns: TypeSprint},
	{Name: "releasedVersions", Returns: TypeVersion},
	{Name: "unreleasedVersions", Returns: TypeVersion},
	{Name: "currentProject", Returns: TypeProject},
})

func newFuncRegistry(specs []FuncSpec) *funcRegistry {
	r := &funcRegistry{specs: make(map[string]FuncSpec, len(specs))}
	for _, s := range specs {
		r.specs[strings.ToLower(s.Name)] = s
		r.names = append(r.names, s.Name)
	}
	sort.Strings(r.names)
	return r
}

// Lookup returns the signature for name, ignoring case.
func (r *funcRegistry) Lookup(name string) (FuncSpec, bool) {
	s, ok := r.specs[strings.ToLower(name)]
	return s, ok
}

// Names returns the function names in sorted order.
func (r *funcRegistry) Names() []string {
	return append([]string(nil), r.names...)
}

// compatible reports whether a value of type ret can be compared with a field of type field.
// Date and datetime are interchangeable.
func compatible(field, ret FieldType) bool {
	if field == ret {
		return true
	}
	return (field == TypeDate && ret == TypeDateTime) || (field == TypeDateTime && ret == TypeDate)
}
