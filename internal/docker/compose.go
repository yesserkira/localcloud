package docker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Controller manages Docker Compose operations for the project.
type Controller struct {
	files       []string
	projectName string
	workDir     string
	logger      *slog.Logger
}

// NewController creates a Docker Compose controller.
func NewController(files []string, projectName, workDir string, logger *slog.Logger) *Controller {
	return &Controller{
		files:       files,
		projectName: projectName,
		workDir:     workDir,
		logger:      logger,
	}
}

// Up starts Docker Compose services.
func (c *Controller) Up(ctx context.Context) error {
	args := c.baseArgs()
	args = append(args, "up", "-d", "--wait")
	c.logger.Info("docker compose up", "project", c.projectName)
	return c.run(ctx, args)
}

// Down stops Docker Compose services.
func (c *Controller) Down(ctx context.Context) error {
	args := c.baseArgs()
	args = append(args, "down")
	c.logger.Info("docker compose down", "project", c.projectName)
	return c.run(ctx, args)
}

// PS returns the status output of running services.
func (c *Controller) PS(ctx context.Context) (string, error) {
	args := c.baseArgs()
	args = append(args, "ps", "--format", "table {{.Name}}\t{{.Status}}\t{{.Ports}}")
	return c.output(ctx, args)
}

// IsHealthy checks if all services report healthy.
func (c *Controller) IsHealthy(ctx context.Context) (bool, error) {
	args := c.baseArgs()
	args = append(args, "ps", "--format", "{{.Health}}")
	out, err := c.output(ctx, args)
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line != "healthy" && line != "" {
			return false, nil
		}
	}
	return true, nil
}

func (c *Controller) baseArgs() []string {
	var args []string
	for _, f := range c.files {
		args = append(args, "-f", f)
	}
	if c.projectName != "" {
		args = append(args, "-p", c.projectName)
	}
	return args
}

func (c *Controller) run(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = c.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	c.logger.Debug("exec", "cmd", cmd.String())

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose: %w: %s", err, stderr.String())
	}
	return nil
}

func (c *Controller) output(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = c.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
