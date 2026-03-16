package types

type DocType int

const (
	Web DocType = iota
	Local
)

var DocTypeNames = map[string]DocType{
	"web":   Web,
	"file":  Local,
	"local": Local,
}
