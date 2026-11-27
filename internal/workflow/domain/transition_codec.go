package domain

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// EncodeTransition serializes one transition with the shape Encode uses for each entry of the workflow
// envelope. The administration API uses it to show transitions without the surrounding envelope.
func EncodeTransition(t Transition) ([]byte, error) {
	td, err := encodeTransition(t)
	if err != nil {
		return nil, err
	}
	return json.Marshal(td)
}

// DecodeTransition parses one transition in the shape EncodeTransition writes. Unknown fields and unknown
// rule types are rejected; semantic checks are left to Workflow.Validate.
func DecodeTransition(data []byte) (Transition, error) {
	var td transitionDTO
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&td); err != nil {
		return Transition{}, apperror.WrapWithCode(apperror.KindValidation, CodeMalformed, ErrMalformed.Message, err)
	}
	if _, err := d.Token(); err != io.EOF {
		return Transition{}, ErrMalformed
	}
	return decodeTransition(td)
}

// encodeTransition converts one transition into its DTO.
func encodeTransition(t Transition) (transitionDTO, error) {
	td := transitionDTO{ID: t.ID, Name: t.Name, From: nonNil(t.From), To: t.To, RequiredFields: nonNil(t.RequiredFields)}
	var err error
	if td.Conditions, err = encodeRules(t.Conditions); err != nil {
		return transitionDTO{}, err
	}
	if td.Validators, err = encodeRules(t.Validators); err != nil {
		return transitionDTO{}, err
	}
	if td.PostFunctions, err = encodeRules(t.PostFunctions); err != nil {
		return transitionDTO{}, err
	}
	return td, nil
}

// decodeTransition converts a DTO back into a transition, decoding every rule through its registry.
func decodeTransition(td transitionDTO) (Transition, error) {
	t := Transition{ID: td.ID, Name: td.Name, From: td.From, To: td.To, RequiredFields: td.RequiredFields}
	if len(t.From) == 0 {
		t.From = nil
	}
	if len(t.RequiredFields) == 0 {
		t.RequiredFields = nil
	}
	for _, r := range td.Conditions {
		c, err := DecodeCondition(r.Type, r.Params)
		if err != nil {
			return Transition{}, err
		}
		t.Conditions = append(t.Conditions, c)
	}
	for _, r := range td.Validators {
		v, err := DecodeValidator(r.Type, r.Params)
		if err != nil {
			return Transition{}, err
		}
		t.Validators = append(t.Validators, v)
	}
	for _, r := range td.PostFunctions {
		p, err := DecodePostFunction(r.Type, r.Params)
		if err != nil {
			return Transition{}, err
		}
		t.PostFunctions = append(t.PostFunctions, p)
	}
	return t, nil
}
