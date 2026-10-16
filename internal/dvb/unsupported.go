//go:build !linux || (!amd64 && !arm64)

package dvb

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rokoucha/streamisdvb/internal/config"
)

func Receive(context.Context, config.Config, *slog.Logger) error {
	return errors.New("requires Linux amd64 or arm64")
}
