package request

type TaskExec struct {
	Name string `json:"name" validate:"required"`
	Code string `json:"code" validate:"required"`
}

type TaskPID struct {
	PID uint64 `json:"pid" query:"pid" validate:"required"`
}
