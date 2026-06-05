package seeding

import (
	"fmt"
	"sync/atomic"
)

var seedingTestDBCounter uint64

func uniqueSQLiteDSN() string {
	n := atomic.AddUint64(&seedingTestDBCounter, 1)
	return fmt.Sprintf("file:seedtest_%d?mode=memory&cache=shared", n)
}
