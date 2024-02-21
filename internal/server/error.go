package server

type MarketServerError string

func (err MarketServerError) Error() string {
	return string(err)
}
