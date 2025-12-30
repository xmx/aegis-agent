package request

type TaskExec struct {
	Name string `json:"name" validate:"required"`
	Code string `json:"code" validate:"required"`
}
