package mockapi

import (
	"context"
	"fmt"
	"time"
)

// DownloadFile simulates a file download. It returns the whole content as []byte.
func DownloadFile(ctx context.Context, url string) ([]byte, error) {
	if err := simulateWork(ctx, 1000*time.Millisecond); err != nil {
		return nil, err
	}

	return fmt.Appendf(nil, "This is the content of %s", url), nil
}
