package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	QueueGeneratePlan = "queue:generate_plan"
	QueueProcessClip  = "queue:process_clip"
	QueueRenderFinal  = "queue:render_final"
)

type Queue struct {
	client *redis.Client
}

type Job struct {
	ID        uuid.UUID              `json:"id"`
	Type      string                 `json:"type"`
	ProjectID uuid.UUID              `json:"project_id"`
	ClipID    *uuid.UUID             `json:"clip_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

func New(redisURL string) (*Queue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Queue{client: client}, nil
}

func (q *Queue) Close() error {
	return q.client.Close()
}

func (q *Queue) Enqueue(ctx context.Context, queueName string, job *Job) error {
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return q.client.RPush(ctx, queueName, data).Err()
}

func (q *Queue) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Job, error) {
	result, err := q.client.BLPop(ctx, timeout, queueName).Result()
	if err == redis.Nil {
		return nil, nil // No job available
	}
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue: %w", err)
	}

	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected redis response")
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

func (q *Queue) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	return q.client.LLen(ctx, queueName).Result()
}

// EnqueueGeneratePlan enqueues a plan generation job
func (q *Queue) EnqueueGeneratePlan(ctx context.Context, projectID uuid.UUID, jobID uuid.UUID) error {
	job := &Job{
		ID:        jobID,
		Type:      "generate_plan",
		ProjectID: projectID,
	}
	return q.Enqueue(ctx, QueueGeneratePlan, job)
}

// EnqueueProcessClip enqueues a clip processing job
func (q *Queue) EnqueueProcessClip(ctx context.Context, projectID, clipID, jobID uuid.UUID) error {
	job := &Job{
		ID:        jobID,
		Type:      "process_clip",
		ProjectID: projectID,
		ClipID:    &clipID,
	}
	return q.Enqueue(ctx, QueueProcessClip, job)
}

// EnqueueRenderFinal enqueues a final video rendering job
func (q *Queue) EnqueueRenderFinal(ctx context.Context, projectID, jobID uuid.UUID) error {
	job := &Job{
		ID:        jobID,
		Type:      "render_final",
		ProjectID: projectID,
	}
	return q.Enqueue(ctx, QueueRenderFinal, job)
}
