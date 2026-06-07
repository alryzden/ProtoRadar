package cli

const timeFormat = "2006-01-02T15:04:05Z07:00"

type ExitError struct {
	Code int
	Err  error
}

func (err ExitError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err ExitError) Unwrap() error {
	return err.Err
}
