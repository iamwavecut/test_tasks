package expires

import (
	"time"
)

const (
	Never  time.Duration = -1
	Second               = time.Second
	Minute               = Second * 60
	Hour                 = Minute * 60
	Day                  = Hour * 24
	Week                 = Day * 7
)
