package service

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
)

type ValidationService[K any, V any] struct {
	validator *validator.Validate
}

func NewValidationService[K any, V any]() *ValidationService[K, V] {
	return &ValidationService[K, V]{
		validator: validator.New(),
	}
}

func (v *ValidationService[K, V]) Validate(entity K) error {
	// Validate the entity using the validator
	err := v.validator.Struct(entity)
	if err != nil {
		return err
	}
	return nil
}

func (v *ValidationService[K, V]) ValidateUpdateRequest(payload map[string]interface{}) (map[string]interface{}, error) {
	// Convert payload to JSON then to struct
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Unmarshal into the update schema
	var updateSchema V
	err = json.Unmarshal(jsonData, &updateSchema)
	if err != nil {
		return nil, err
	}

	// Validate the update schema
	err = v.validator.Struct(updateSchema)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
