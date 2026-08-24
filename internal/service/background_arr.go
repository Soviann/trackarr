package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Soviann/trackarr/internal/model"
)

func (w *TaskQueueWorker) handleArrPush(ctx context.Context, task model.Task, logger *slog.Logger, app string) error {
	var payload PushPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode arr_push payload: %w", err)
	}

	if w.arrSvc == nil {
		return fmt.Errorf("arr service not configured")
	}

	_, err := w.arrSvc.PushTitle(ctx, payload.TitleID, payload)
	return err
}
