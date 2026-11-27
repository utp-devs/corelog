package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// FormatVersion is the version of the JSON envelope written by Encode.
const FormatVersion = 1

type ruleDTO struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`
}

type statusDTO struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category Category `json:"category"`
}

type transitionDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	From           []string  `json:"from"`
	To             string    `json:"to"`
	Conditions     []ruleDTO `json:"conditions"`
	Validators     []ruleDTO `json:"validators"`
	PostFunctions  []ruleDTO `json:"post_functions"`
	RequiredFields []string  `json:"required_fields"`
}

type workflowDTO struct {
	Version     int             `json:"version"`
	Initial     string          `json:"initial"`
	Statuses    []statusDTO     `json:"statuses"`
	Transitions []transitionDTO `json:"transitions"`
}

func decodeParams[T any](raw json.RawMessage) (T, error) {
	var out T
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, apperror.WrapWithCode(apperror.KindValidation, CodeMalformed, "los parámetros de la regla están mal formados", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return out, ErrMalformed
	}
	return out, nil
}

type decoder[R Rule] func(json.RawMessage) (R, error)

func decoderOf[T any, R Rule](convert func(T) R) decoder[R] {
	return func(raw json.RawMessage) (R, error) {
		v, err := decodeParams[T](raw)
		return convert(v), err
	}
}

// The registries are read-only after package initialization.
var conditionDecoders = map[string]decoder[Condition]{
	"user_in_role": decoderOf(func(v UserInRole) Condition { return v }),
	"is_assignee":  decoderOf(func(v IsAssignee) Condition { return v }),
	"is_reporter":  decoderOf(func(v IsReporter) Condition { return v }),
	"permission":   decoderOf(func(v Permission) Condition { return v }),
	"field_equals": decoderOf(func(v FieldEquals) Condition { return v }),
}

var validatorDecoders = map[string]decoder[Validator]{
	"required_field":      decoderOf(func(v RequiredField) Validator { return v }),
	"comment_required":    decoderOf(func(v CommentRequired) Validator { return v }),
	"resolution_required": decoderOf(func(v ResolutionRequired) Validator { return v }),
	"subtasks_done":       decoderOf(func(v SubtasksDone) Validator { return v }),
	"assignee_set":        decoderOf(func(v AssigneeSet) Validator { return v }),
}

var postFunctionDecoders = map[string]decoder[PostFunction]{
	"set_resolution":   decoderOf(func(v SetResolution) PostFunction { return v }),
	"clear_resolution": decoderOf(func(v ClearResolution) PostFunction { return v }),
	"assign":           decoderOf(func(v Assign) PostFunction { return v }),
	"set_field":        decoderOf(func(v SetField) PostFunction { return v }),
	"add_comment":      decoderOf(func(v AddComment) PostFunction { return v }),
	"emit":             decoderOf(func(v Emit) PostFunction { return v }),
}

func lookup[R Rule](registry map[string]decoder[R], typ string, params json.RawMessage) (R, error) {
	d, ok := registry[typ]
	if !ok {
		var zero R
		return zero, apperror.WithCode(apperror.KindValidation, CodeUnknownRuleType, fmt.Sprintf("tipo de regla desconocido: %q", typ))
	}
	return d(params)
}

// DecodeCondition builds a condition from its type tag and JSON parameters.
func DecodeCondition(typ string, params json.RawMessage) (Condition, error) {
	return lookup(conditionDecoders, typ, params)
}

// DecodeValidator builds a validator from its type tag and JSON parameters.
func DecodeValidator(typ string, params json.RawMessage) (Validator, error) {
	return lookup(validatorDecoders, typ, params)
}

// DecodePostFunction builds a post-function from its type tag and JSON parameters.
func DecodePostFunction(typ string, params json.RawMessage) (PostFunction, error) {
	return lookup(postFunctionDecoders, typ, params)
}

// RuleTypes lists the registered type tags per kind, sorted.
func RuleTypes() (conditions, validators, postFunctions []string) {
	return sortedKeys(conditionDecoders), sortedKeys(validatorDecoders), sortedKeys(postFunctionDecoders)
}

func sortedKeys[R Rule](m map[string]decoder[R]) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func encodeRules[R Rule](rules []R) ([]ruleDTO, error) {
	out := []ruleDTO{}
	for _, rule := range rules {
		// A nil interface value cannot be encoded.
		if any(rule) == nil {
			return nil, ErrMalformed
		}
		params, err := json.Marshal(rule)
		if err != nil {
			return nil, err
		}
		out = append(out, ruleDTO{Type: rule.Type(), Params: params})
	}
	return out, nil
}

func nonNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

// Encode serializes the workflow into the versioned JSON envelope. Output is stable.
func Encode(w Workflow) ([]byte, error) {
	dto := workflowDTO{Version: FormatVersion, Initial: w.Initial, Statuses: []statusDTO{}, Transitions: []transitionDTO{}}
	for _, s := range w.Statuses {
		dto.Statuses = append(dto.Statuses, statusDTO(s))
	}
	for _, t := range w.Transitions {
		td, err := encodeTransition(t)
		if err != nil {
			return nil, err
		}
		dto.Transitions = append(dto.Transitions, td)
	}
	return json.Marshal(dto)
}

// Decode parses the versioned JSON envelope. It rejects unknown fields, unknown versions
// and unknown rule types, but does not run semantic validation (see Workflow.Validate).
func Decode(data []byte) (Workflow, error) {
	var dto workflowDTO
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&dto); err != nil {
		return Workflow{}, apperror.WrapWithCode(apperror.KindValidation, CodeMalformed, ErrMalformed.Message, err)
	}
	if _, err := d.Token(); err != io.EOF {
		return Workflow{}, ErrMalformed
	}
	if dto.Version != FormatVersion {
		return Workflow{}, ErrUnsupportedVersion
	}
	w := Workflow{Initial: dto.Initial}
	for _, s := range dto.Statuses {
		w.Statuses = append(w.Statuses, Status(s))
	}
	for _, td := range dto.Transitions {
		t, err := decodeTransition(td)
		if err != nil {
			return Workflow{}, err
		}
		w.Transitions = append(w.Transitions, t)
	}
	return w, nil
}
