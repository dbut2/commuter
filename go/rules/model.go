package rules

type Cond struct {
	ID    string
	Field string
	Op    string
	Value string
}

type Act struct {
	ID       string
	Key      string
	Template string
}

type Rule struct {
	ID      string
	Name    string
	Enabled bool
	Conds   []Cond
	Acts    []Act
}
