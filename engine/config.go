package engine

import (
	"os"
)

type Config struct {
	HTTPBindingPoint                 string
	TelnetBindingPoint               string
	Storage                          *os.File
	StorageDeferredWriteIntervalSecs int
	TTLCheckIntervalSecs             int
	Debug                            bool
}
