package interfaces

type ValidatorInterface[K any, V any] interface {
	Validate(entity K) error
	ValidateUpdateRequest(payload map[string]interface{}) (map[string]interface{}, error)
}
