package repository

type Repo interface {
	SeValue(ID string, value any) error
	GetValue(ID string) (any, error)
	AllValues() ([]any, error)
}
