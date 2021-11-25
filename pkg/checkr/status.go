package checkr

type Status string

func (s Status) String() string {
	return string(s)
}

const (
	SSNTracePending  Status = "pending"
	SSNTraceClear           = "clear"
	SSNTraceConsider        = "consider"
)
