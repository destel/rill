package mockapi

import (
	"context"
	"fmt"
	"time"
)

// DownloadFile simulates a file download. It returns the whole content as []byte.
func DownloadFile(ctx context.Context, url string) ([]byte, error) {
	randomSleep(ctx, 1000*time.Millisecond)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("This is the content of %s", url)), nil
}
