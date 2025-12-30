package request

type QueryNames struct {
	Name []string `json:"name" query:"name" validate:"gte=1,lte=100,dive,required"`
}

func (qns QueryNames) Get() string {
	if len(qns.Name) > 0 {
		return qns.Name[0]
	}

	return ""
}
