package global

type Log struct {
	App     string
	Tag     string
	LogLine string
}

type LogStats map[string]map[int64]interface{}
